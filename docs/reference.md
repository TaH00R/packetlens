# PacketLens

PacketLens is a local network observability dashboard built with Go, Next.js,
React, and TypeScript. It runs TCP connection and DNS lookup probes, stores
complete monitoring cycles in SQLite, and streams snapshots to the dashboard.
Use it only for destinations you own or are authorized to monitor.

## Features

- Overview, TCP Analytics, DNS Analytics, Network Health, Failure Incidents, and a metrics guide.
- Exact P50/P90/P95/P99 successful latencies, distributions, target comparisons, and prior-period comparisons.
- Per-target mean and latest-pair jitter, with historical trends.
- Persistent rolling median/MAD anomaly episodes, duplicate suppression, a feed, and chart markers.
- Sustained-failure incidents, confirmed recovery, elapsed spans, and a dedicated incident timeline.
- Persisted cross-target failure correlation with possible shared connectivity indicators.
- Observed availability for 1h, 24h, 7d, and 30d, with outcome streaks and timestamps.
- Direct DNS resolver comparisons with persisted A-query outcomes, latency trends and timeout counts.
- Aggregate and per-target latency charts; failures appear as gaps.
- Global ranges: last 5 minutes, 15 minutes, hour, 6 hours, 24 hours, or All.
- Durable SQLite snapshots and probe results, with migrations and configurable retention.
- Bounded REST history pages with time/type/target filters and cursor pagination.
- REST history hydration at startup and on WebSocket connection/reconnection.
- Validated payloads, deduplication, chronological merging, and automatic reconnects
  with a delay from 1 to 15 seconds.
- Loading, history-error, connection, empty, and stale measurement messages.
- Authenticated target management with persisted per-target intervals/timeouts and bounded scheduling.
- Streaming CSV exports for measurements, resolver comparisons and incident history.
- Desktop sidebar and mobile navigation.

## Stack and layout

| Area | Technology |
| --- | --- |
| Backend | Go, `github.com/coder/websocket` |
| Frontend | Next.js App Router, React, TypeScript |
| Styling | Tailwind CSS |
| Charts / icons | Recharts / Lucide React |
| Storage | SQLite through `database/sql`; pure-Go driver, no C compiler required |

```text
packetlens/
|-- backend/
|   |-- cmd/packetlens/main.go
|   |-- internal/
|   |   |-- analytics/    # Persisted latency, jitter, and availability
|   |   |-- anomaly/      # Pure rolling median/MAD detector and episode policy
|   |   |-- correlation/  # Recent cross-target failure overlap episodes
|   |   |-- incident/     # Sustained observed failure and recovery state machine
|   |   |-- api/          # HTTP and WebSocket handlers
|   |   |-- monitor/      # Complete monitoring cycles and types
|   |   |-- target/       # Validated configuration and serialized management
|   |   |-- resolver/     # Explicit resolver endpoint configuration
|   |   |-- probe/        # TCP and DNS checks
|   |   |-- realtime/     # Client registration and snapshot broadcast
|   |   |-- store/        # Storage interface, SQLite, migrations, retention, test memory store
|   |-- go.mod
|-- frontend/
|   |-- app/              # App Router pages; there is no src/ directory
|   |-- components/
|   |-- context/          # Shared monitoring state and time range
|   |-- hooks/            # WebSocket lifecycle
|   |-- lib/              # Payload validation, merging, filtering
|   |-- tests/
|   |-- types/
|   |-- package.json
|-- AGENTS.md
|-- README.md
```

## Run locally

Requirements: Go 1.27.1 or newer (see `backend/go.mod`), Node.js 24.15 or
newer for the frontend's direct TypeScript test runner, and npm. The backend
needs network access to its configured destinations.

```bash
git clone https://github.com/TaH00R/packetlens.git
cd packetlens
```

Start the backend in one terminal:

```bash
cd backend
go mod download
go run ./cmd/packetlens
```

Start the frontend in another terminal:

```bash
cd frontend
npm ci
npm run dev
```

Open [http://localhost:3000](http://localhost:3000). The backend listens on
port 8080. Both services must be running for live monitoring.

The frontend defaults to the following public URLs. To override them, create
`frontend/.env.local` using `frontend/.env.example`:

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_WS_URL=ws://localhost:8080/api/ws
```

Restart the development server after changing these variables; production
builds embed them at build time. These URL values are public, not secrets.

REST CORS and WebSocket origins allow the local frontend on
`http://localhost:3000` and `http://127.0.0.1:3000`. If port 3000 is occupied,
free it before starting Next.js; another port will need corresponding backend
origin changes. Setting frontend URLs alone does not change the allowlist.

For a production frontend build:

```bash
cd frontend
npm run build
npm run start
```

This is a local development setup. Remote deployment needs deliberate origin
configuration, HTTPS/WSS, and access controls. Target management requires a private
admin token, but monitoring/history APIs are still unauthenticated; do not expose
the backend directly to the public internet.

## Monitoring and configuration

Default configuration (targets are seeded once by migration 005):

| Setting | Current value |
| --- | --- |
| Default per-target interval | 5 seconds |
| TCP targets | `1.1.1.1:443`, `8.8.8.8:443` |
| DNS domains | `google.com`, `cloudflare.com` |
| Backend storage | SQLite at `data/packetlens.db` |
| History retention | 720 hours (30 days) |
| Cleanup interval | 1 hour, including a sweep at startup |
| Default history page | Latest 1,000 cycles |
| HTTP address | `:8080` (in `internal/api/server.go`) |

Targets, enabled state, intervals, and timeouts are editable through `/targets`
when management is enabled. Their persisted values replace the defaults across
restarts. The HTTP address still requires a code change. Both probe types default
to a 3-second timeout and respect parent cancellation.

Storage configuration is read from process environment variables at startup:

| Variable | Default | Validation |
| --- | --- | --- |
| `PACKETLENS_DB_PATH` | `data/packetlens.db` | Nonempty filesystem path; parent directories are created |
| `PACKETLENS_RETENTION` | `720h` | Go duration between 1h and 87600h |
| `PACKETLENS_CLEANUP_INTERVAL` | `1h` | Between 1m and 24h, no greater than retention |

Relative paths resolve from the backend process working directory. Use an absolute
path when launching from different directories so restarts open the same database.
`backend/.env.example` documents these settings; the backend does not automatically
load dotenv files. For example, in PowerShell before `go run ./cmd/packetlens`:

```powershell
$env:PACKETLENS_DB_PATH = 'D:\packetlens-data\history.db'
$env:PACKETLENS_RETENTION = '720h'
$env:PACKETLENS_CLEANUP_INTERVAL = '1h'
```

Use `720h`, not `30d`: Go duration syntax does not accept days. Invalid settings,
unreadable/corrupt databases, and unsupported newer schema versions stop startup
with an error. The application never silently replaces a damaged database.

A cycle groups the targets due at dispatch and waits for all selected probes before
storing and broadcasting a snapshot. Different per-target intervals can mean only
a subset of enabled targets is included. No probe result is fabricated for targets
that are not due.
The snapshot and its probe rows are committed together in one transaction.
Stored snapshots have an additive `id` field shared by REST and live messages.
The collector schedules enabled targets immediately at startup, then uses each
target's interval. Slow
WebSocket clients are disconnected when their 64-snapshot queue fills; writes
have a 5-second timeout. Queue drops and write failures are logged. Shutdown
cancels collection and pending database work, closes client queues, waits for
background workers, and closes the database.

A runtime database write failure is logged and live streaming continues without
claiming that the failed cycle was persisted. Such samples have no stable ID and
cannot be recovered after a page reload. Historical-query failures return a generic
JSON error with status 503. Health also returns 503 when the database is unreachable.

## API

| Method | Endpoint | Response |
| --- | --- | --- |
| GET | `/api/export/measurements.csv` | Streaming filtered TCP/system DNS measurements |
| GET | `/api/export/dns-resolvers.csv` | Streaming explicit DNS resolver measurements |
| GET | `/api/export/incidents.csv` | Streaming overlapping incident records |
| GET | `/api/health` | `status`, `service`, `timestamp` |
| GET | `/api/metrics` | `count`, `results` (flattened probes) |
| GET | `/api/stats` | `total_results`, `probes` (statistics by type) |
| GET | `/api/analytics` | Full-range latency, jitter, histogram, trends, previous period |
| GET | `/api/anomalies` | Bounded, filtered anomaly episode feed |
| GET | `/api/incidents` | Incident history, filtered pages, open/recovered totals |
| GET | `/api/correlations` | Bounded possible shared connectivity event feed |
| GET / POST | `/api/targets` | Authenticated target list / creation |
| PUT / DELETE | `/api/targets/{id}` | Authenticated replacement / removal |
| GET | `/api/dns/resolvers` | Resolver comparisons for the selected retained range |
| GET | `/api/availability` | Fixed-window observed availability, streaks, last outcomes |
| GET | `/api/snapshots` | `count`, `snapshots`, `has_more`, `next_cursor` |
| WebSocket | `/api/ws` | One snapshot per new monitoring cycle |

```bash
curl http://localhost:8080/api/health
curl http://localhost:8080/api/snapshots
```

Health checks HTTP service/database connectivity; it does not establish network
health, guarantee that a future write will succeed, or guarantee any probe succeeded. An empty store returns empty arrays for
metrics and snapshots and an empty `probes` object for statistics.

Snapshot example (illustrative values):

```json
{
  "timestamp": "2026-11-06T08:30:00Z",
  "tcp": [{
    "type": "tcp",
    "target": "1.1.1.1:443",
    "latency_ms": 24.6,
    "success": true,
    "timestamp": "2026-11-06T08:30:00Z"
  }],
  "dns": []
}
```

Failed probes include an optional `error` string and `success: false`.
Their recorded attempt duration is excluded from successful latency summaries.
Statistics include `total`, `successful`, `failed`, `avg_latency_ms`,
`min_latency_ms`, and `max_latency_ms`. Backend latency statistics are zero
when there are no successful probes; consult the counts before interpreting
those values. New analytics return null for unavailable latency statistics; frontend cards show No data when no successes exist.

## Persistent history and bounded queries

SQLite stores snapshots and individual probe results with indexed timestamps,
targets, and types. Embedded numbered SQL migrations run transactionally at
startup, with schema version recorded in `PRAGMA user_version`. WAL mode,
foreign keys, a five-second busy timeout, and full synchronous commits are enabled.
The local connection pool is limited to four connections so analytics readers can
run alongside collection. SQLite temporary sorting uses file-backed storage.
Keep the database on a local disk, not a network filesystem.

Restarting the backend preserves committed data. Existing process-local history
from the old MemoryStore cannot be recovered or automatically imported. Stop the
backend before copying its database for a backup; do not copy just the main file
while it is running because committed changes may still be in its WAL sidecar.

Cleanup deletes cycles older than the retention cutoff, together with their probe
results, in batches of at most 500 cycles. The exact cutoff is retained. Startup
and periodic sweeps run in the background; expired data may remain until a sweep
finishes. Cleanup failures are logged and retried at the next interval. Cleanup
makes database pages reusable; it does not guarantee immediate file shrinkage.

`GET /api/snapshots` and `GET /api/metrics` accept:

| Parameter | Meaning |
| --- | --- |
| `from`, `to` | Optional inclusive RFC3339 timestamps, based on snapshot time |
| `type` | Optional `tcp` or `dns` |
| `target` | Optional exact target match |
| `limit` | Cycles per page, 1 to 1,000; default 1,000 |
| `before` | Opaque `next_cursor` from the previous page |

The first page contains the latest matching cycles, returned chronologically.
To walk backward, pass `next_cursor` as `before` with the same filters until
`has_more` is false. Cursors use timestamp and stable ID, so equal timestamps
and newly arriving live samples do not shift the older page boundary. Retention
can remove records between requests; pagination is not a frozen database export.

Snapshot type/target filters select matching cycles but still return every probe
in those cycles. Metrics applies the same cycle selection and then emits only
matching probe results. Its `limit` therefore counts cycles, not individual results.
`count` always describes the returned array, not a total database count. Pagination
metadata is additive; original response fields and snapshot messages remain valid.
Unknown, repeated, empty, or malformed query parameters receive JSON errors (400).

For example:

```bash
curl "http://localhost:8080/api/snapshots?from=2026-11-06T00:00:00Z&to=2026-11-07T00:00:00Z&limit=100"
curl "http://localhost:8080/api/metrics?type=dns&target=google.com&limit=100"
```

`GET /api/stats` accepts `from`, `to`, `type`, and `target` and aggregates all
matching retained results in SQLite without loading their raw history into Go.
It does not accept pagination parameters. Failed probes are excluded from latency
aggregates; its existing count and latency fields are preserved.

## Persisted analytics

`GET /api/analytics?range=15m` accepts `range` (5m, 15m, 1h, 6h, 24h, all)
and an optional RFC3339 `to` timestamp (years 1970 through 9998). Defaults are
15m ending at server request time. It returns:

- `range`, `as_of`, and the earliest retained snapshot timestamp `history_start`.
- `current`: `from`, `to`, summaries for TCP/DNS and each target, histograms,
  group trends, and per-target trends.
- `previous`: the preceding equal-length period for finite ranges, without
  duplicated boundary samples. It is null for All because no equal prior window
  is implied by selecting all retained history.

The current selection includes both endpoints. The previous period includes its
lower endpoint and ends immediately before the current lower endpoint. Range
membership uses snapshot timestamps. Within each target, jitter orders successes
by probe timestamp, then snapshot timestamp, stable ID, and probe ordinal for ties.
Only successful measurements contribute to latency, percentiles, or jitter.

Percentiles are exact nearest ranks: sort the successful latencies and select the
one-based rank `ceil(p / 100 * n)`. There is no interpolation. With one success,
every percentile equals that measurement; no successes means null. Histograms
use bins [0,10), [10,25), [25,50), [50,100), [100,250), [250,500), [500,1000),
and [1000,infinity), all in milliseconds.

Trend output has 60 equal-duration buckets across the entire selection. Every
measurement contributes to its bucket; there is no latest-60-samples truncation.
Latency trends report successful means. Empty or all-failed buckets have null
latency, while an all-failed observed bucket has 0% availability. Empty buckets
have null availability. This aggregation can hide short-lived latency extremes;
summary maxima and exact percentiles remain based on every selected observation.
Jitter differences are assigned to the bucket containing the newer observation,
including pairs that cross bucket boundaries. Group jitter trends average valid
per-target pair differences in each bucket; group summary jitter is an unweighted
average of the targets' mean jitters. These two aggregations have different weights.
`current_jitter_ms` is the latest valid pair within the selection for each target,
not a statement that a recently failed target currently has valid latency.

`GET /api/availability` accepts only the optional `to` timestamp. It returns
`as_of`, `history_start`, and TCP/DNS and target summaries with fixed 1h, 24h, 7d,
and 30d windows. Each includes attempts, successes, percentage, and first/last
observed snapshot timestamps; type summaries include 60-bucket trends. Target
summaries include last success/failure probe timestamps and consecutive observed
success/failure counts within the retained 30-day query. Streaks may span monitoring
gaps; they are neither uninterrupted uptime nor an inference about unmeasured time.
No attempts means null availability. Short or partially retained histories are
reported as observed counts rather than extended into assumed successes.

Both endpoints stream query rows into bounded summaries rather than loading all
raw results into Go/browser memory. SQLite computes success ranks with window
functions; sorting may use temporary disk. Reports support at most 128 distinct
probe-type/target identities; exceeding that limit returns an error, never a partial
report. There is a 30-second deadline and at most two concurrent analytics requests
per server; excess requests return JSON 429 with Retry-After. Other database errors
return generic JSON 503 responses and contextual logs. No new production dependency
was added for analytics.

## Statistical latency anomaly detection

The backend now runs a detector independently for each probe type and exact target.
No external AI service is used. Detection starts with new measurements after the
schema-v2 upgrade; old history is preserved but not retrospectively classified.

Before evaluating a successful probe, take up to the configured window of preceding
accepted normal successes, excluding samples older than the maximum baseline age.
The current probe is excluded. With enough samples:

```text
baseline = median(previous normal latencies)
MAD = median(abs(previous normal latency - baseline))
threshold = baseline + max(minimum_delta_ms, MAD_multiplier * MAD)
anomaly = current successful latency > threshold
```

For even sample counts, median is the average of the two middle values. MAD is
used directly, without a normal-distribution scale factor; the multiplier is in
MAD units, not standard deviations or a confidence probability. The absolute
floor prevents zero-MAD baselines from flagging tiny fluctuations. A probe exactly
at the threshold is normal. Failures neither enter the baseline nor become zero-ms
latency observations. Out-of-order probe timestamps are stored as raw history but
do not advance detector state or create retrospective episodes.

Runtime configuration (process environment, also in `backend/.env.example`):

| Variable | Default | Valid range |
| --- | --- | --- |
| `PACKETLENS_ANOMALY_WINDOW` | 60 | 3..500 accepted normal successes |
| `PACKETLENS_ANOMALY_MIN_SAMPLES` | 20 | 3..window |
| `PACKETLENS_ANOMALY_MAD_MULTIPLIER` | 6 | Finite, 1..100 |
| `PACKETLENS_ANOMALY_MIN_DELTA_MS` | 10 | Finite, 0.1..60000 ms |
| `PACKETLENS_ANOMALY_MAX_AGE` | 15m | 1m..24h, Go duration syntax |
| `PACKETLENS_ANOMALY_RECOVERY_SAMPLES` | 3 | 1..100 consecutive normal successes |

At the five-second default cadence, a new target usually needs about 100 seconds
of successful observations before its first evaluation. Baselines and recovery
progress survive restarts. They are persisted in the same transaction as raw
snapshots and episode updates, so a failed write cannot partially advance detection.
Configure baseline age and minimum samples to fit the probe cadence; too few recent
normal successes means no classification.

One open episode is allowed per probe type and target, enforced by a SQLite unique
index. Its threshold stays frozen while open. Further high successes update the
same ID, anomalous-sample count, most recent high timestamp, and peak latency.
Initial latency and threshold remain unchanged. Severity starts as warning; it is
critical when any high latency reaches `baseline + 2 * (threshold - baseline)`.
Severity only escalates within an episode. These categories describe latency
excursions, not confirmed outages or causes.

Recovery requires consecutive successful probes at or below that frozen threshold.
A failure or another high latency resets recovery progress. A gap longer than the
currently configured baseline age resets partial recovery; missing time does not
recover an episode. Confirmed recovery records both probe and cycle timestamps.
Recovery successes can then enter the normal baseline. Subsequent high latency can
open a new episode after sufficient normal samples exist. Existing episodes keep
their creation-time threshold and required recovery count if startup settings
change; current window/age settings govern baseline aging and gap handling.

Open episodes and their bounded state survive raw-history retention. Cleanup
removes up to 500 recovered episodes and 500 inactive detector states per batch,
based on recovery/last-observation age, alongside raw snapshot cleanup. In a
backlog with no expired snapshots, remaining episode/state batches may wait until
the next periodic sweep. Closed episode records are independent of raw snapshot
rows and may therefore outlive their onset measurement until recovery ages out.

### Feed and chart behaviour

`GET /api/anomalies` accepts `range` (the same global ranges, default 15m), optional
`to`, `type`, exact `target`, `limit` (1..200, default 50), and `before` (the prior
`next_cursor`). It returns `range`, `as_of`, `count`, `anomalies`, `has_more`, and
`next_cursor`. Counts describe the returned page. Results are newest onset first,
with stable cycle-time/ID cursor pagination. Keep filters and `to` fixed when
walking older pages. Unknown, empty, repeated, or malformed parameters return
JSON 400; storage failures return generic JSON 503.

An episode overlaps a range when it began no later than the upper bound and has
not recovered before the lower bound. An old ongoing episode therefore remains
visible. Status and last-observation details reflect the latest stored episode,
even when querying a historical window; `as_of` is the requested window endpoint,
not a frozen historical version of the record.

The shared context refreshes the newest feed page every 15 seconds and on socket
reconnection. The dedicated `/anomalies` page provides a chronological feed,
probe-type filter, policy details, and working older/previous/latest pagination.
Older-page navigation keeps the original window end fixed. Empty feeds can mean
warmup, insufficient recent samples, or no recorded episodes; they do not establish
network health. Runtime persistence failures are logged and live raw streaming
continues, but failed writes cannot produce durable anomaly records.

Persisted latency trends add `anomaly_count` and nullable `anomaly_latency_ms` per
bucket. Every episode onset in the selection is counted; the displayed red dot is
the maximum initial anomaly latency in that bucket, separately from average
latency. Ongoing updates do not create another onset marker. Raw recent target
charts show exact onset measurements from the newest loaded feed page only and
explicitly label that marker limit. Existing snapshot WebSocket shapes are unchanged.

The method assumes its initial normal baseline is representative. Warmup can
include abnormal conditions, slow changes may adapt into the normal baseline,
and a persistent legitimate step change can remain an open episode because active
thresholds do not learn that plateau. Thresholds need operator tuning; flags are
statistical evidence, not causal diagnoses. There is no acknowledgement/reset UI
for latency anomalies. Sustained-failure incidents use the separate lifecycle below.

## Failure incidents and recovery

Schema v3 adds a failure-incident state machine independently for each exact probe
type and target. Existing snapshots and anomaly state/history are preserved. It
starts with new observations after the upgrade; it does not invent retrospective
incidents from earlier raw measurements.

| Variable | Default | Validation |
| --- | --- | --- |
| `PACKETLENS_INCIDENT_FAILURE_THRESHOLD` | 3 | Integer 1..1000 |
| `PACKETLENS_INCIDENT_RECOVERY_THRESHOLD` | 2 | Integer 1..100 |
| `PACKETLENS_INCIDENT_MAX_GAP` | 30s | Go duration 1s..24h |

These process environment settings are documented in `backend/.env.example`.
Set the maximum gap above the expected observation interval; otherwise a normal
interval may reset every multi-probe streak. Invalid settings stop startup. Policy
is captured when the first failure starts a run: changing settings on restart
affects new runs while pending/open runs keep their saved policy.

A single failed probe is stored and streamed as usual. It begins a bounded pending
candidate, not an incident. A success clears that candidate. Reaching the failure
threshold creates one incident, retaining both the **first failure** (`started_at`)
and **threshold confirmation** (`detected_at`) timestamps and their snapshot-cycle
timestamps. Subsequent failed observations update that incident's failed count,
last failure, and last error. SQLite enforces one open incident per probe type and
target. Errors in incident records are bounded to 1,024 Unicode code points; raw
probe errors retain their existing contract.

An open incident recovers only after the required consecutive successful probes.
A failure resets partial recovery. Success is based on the probe success flag, so
a slow successful probe can contribute to failure-incident recovery while still
belonging to a distinct latency anomaly. Recovery records the confirmation probe
and cycle timestamps. Its `duration_ms` is the elapsed wall-clock span from the
first failed probe to that confirmation. Open records have null duration and no
inferred recovery timestamp; a valid zero-duration recovery remains zero.

A gap **strictly longer** than the saved maximum observation gap resets incomplete
failure and recovery streaks. For an open incident it also records
`has_monitoring_gaps`, preserving the incident until successful confirmation.
Missing time is never counted as a failure or success. An incident's elapsed span
can include unknown periods and does not establish continuous downtime or a root
cause. The timeline warns about recorded gaps and old last-observation timestamps.
Older probe timestamps are stored as raw history but do not advance the detector.
Equal timestamps with distinct observations remain valid.

Pending streaks, open state, recovery progress, raw results, and incident updates
commit atomically with each snapshot. They survive backend restarts. A failed write
or invalid persisted state rolls back the entire cycle; errors are logged and live
raw streaming continues using the existing storage-failure behaviour. No latency
anomaly or incident transition can partially advance in that failed transaction.

Incidents are retained independently of raw snapshots. Open incidents and their
state survive raw-history cleanup; missing measurements cannot silently recover
or delete them. Each cleanup batch can additionally remove up to 500 recovered
incidents whose recovery cycle predates the retention cutoff and 500 inactive
incident-state rows older than the cutoff. As with anomaly cleanup, a backlog may
need subsequent sweeps. Policies and lifecycle details remain attached to retained
records. There are no manual acknowledgement, closure, or root-cause fields in
this milestone; lifecycle changes are driven by observed probes.

### Incident API and timeline

`GET /api/incidents` accepts the same `range`, optional `to`, `type`, exact `target`,
`limit` (1..200, default 50), and `before` parameters as the anomaly feed, plus
`status=all|open|recovered` (default all). Responses include:

- `range`, `as_of`, and the applied `status` filter.
- `incidents`, `count`, `has_more`, and `next_cursor` for the current page.
- `open_count` and `recovered_count` across the whole matching range/type/target,
  independent of pagination and the status filter.

A read transaction keeps the counts and page consistent. Overlap uses snapshot
cycles: incidents that started before the upper bound and have not recovered
before the lower bound qualify. Old open incidents remain visible in recent
ranges. Ordering is by first-failure cycle, newest first, with ID as the stable tie
breaker. Preserve filters and `to` when following cursors. Status is the latest
stored state, including when querying a historical overlap window; this endpoint
is not an immutable historical event log. Bad parameters return JSON 400, closed
or failed storage returns generic JSON 503, and reads have a five-second deadline.

The shared context refreshes the newest unfiltered page every 15 seconds and on
WebSocket reconnection. `/incidents` provides the full timeline; the overview remains
focused on summary cards and latency trends. Desktop and mobile navigation link to the page.
The UI distinguishes ongoing/recovered records, shows first failure, detection,
last failure, recovery progress, confirmed elapsed span, probe errors, and policy.
Status/probe filters and older/previous/latest controls share pagination logic with
the anomaly feed. Changing range or filters aborts the old request and resets page
navigation. Loading, empty, API-error, and stale states are explicit.

The cross-target correlation feed groups recent overlapping failures separately
from these sustained target incidents. Neither establishes a confirmed cause.

## Dashboard history and time ranges

The shared monitoring context loads persisted analytics, anomalies, incidents, correlations, and DNS resolver comparisons
every 15 seconds and availability every minute, also refreshing on WebSocket connection/reconnection.
All pages consume that shared state. Overview summaries and latency trends,
TCP/DNS analytics, and the jitter page use all retained measurements in the global
selection. All means all retained SQLite history for these reports. Availability
uses its explicitly labeled fixed windows independently of the global range.

Target selection controls percentile cards, distributions, and latency/jitter
charts. Comparison tables show all targets in the selected probe type. The UI
shows observation counts, timestamps, partial-history notes, loading/API errors,
and stale-report warnings (45 seconds for analytics, two minutes for availability).
Changing ranges aborts the old request and clears the old range's report until the
new one arrives; a late old response cannot overwrite the selection.

The probe activity page and its target trend tab still use a browser cache
of up to 1,000 merged snapshots, about 83 minutes at the default cadence. Those
sections are explicitly labeled recent and may not cover a long selected range.
The browser validates and deduplicates snapshots by stable ID, with a contents-based
fallback for legacy/live-only payloads, and sorts them chronologically. Changing
the selected range never deletes retained backend history. The cached view advances
on a cleaned-up five-second timer, even without new data. Times are displayed in
the browser's local time zone.

## Metric definitions

All latency and jitter values use milliseconds.

- **TCP latency:** elapsed time to establish a TCP connection. It is not ICMP
  ping or a direct measurement of network round-trip time.
- **DNS latency:** elapsed time for name resolution using the backend machine's
  resolver. Resolver selection and caching affect the result.
- **Average latency:** sum of successful latencies divided by successful count.
  For 20, 25, and 30 ms, the average is 25 ms.
- **Minimum / maximum:** lowest / highest successful latency in the selection.
- **Percentiles:** exact nearest-rank successful latency, as defined above.
- **Latest-pair jitter:** absolute latency difference of the two latest successful
  measurements for one target within the selection. Two successes are required.
- **Probe success rate:** `100 * successful attempts / total attempts`.
  Failure rate is `100 * failed attempts / total attempts`. These are probe
  outcomes, not packet-loss measurements. No attempts means no rate.
- **Per-target jitter:** sort successful measurements for each probe type and
  target by time, take the absolute difference between each successive pair,
  then average those differences. For 20, 30, 25, and 45 ms, jitter is
  `(10 + 5 + 20) / 3 = 11.67 ms`. At least two successes are required.
  Failed attempts are skipped, so a comparison may span a failure gap.
- **Average jitter:** an unweighted average of the per-target jitter values
  that can be calculated. This is latency variation, not RTP interarrival jitter.

A successful probe establishes only that a particular operation succeeded at
that time. It cannot prove that every device, service, or network path is healthy.

## Cross-target failure correlation

Correlation runs once per newly persisted complete monitoring cycle, independently
of the sustained-failure incident thresholds. It can therefore group isolated
failures before individual targets qualify for incidents. Migration 004 adds its
tables automatically at backend startup; existing records remain intact. Old
history is not retrospectively classified.

| Variable | Default | Validation |
| --- | --- | --- |
| `PACKETLENS_CORRELATION_WINDOW` | `15s` | Go duration, 1s..1h |
| `PACKETLENS_CORRELATION_MIN_TARGETS` | `2` | Integer, 2..128 distinct target names |

The detector keeps the latest observed outcome for each probe-type/target identity.
At each evaluation, a failure qualifies when its probe timestamp is within the
inclusive window ending at the evaluation watermark. The watermark is the maximum
of the current cycle timestamp, its probe timestamps, and the prior watermark.
A newer successful outcome supersedes that identity's failure. Distinct exact
target strings are counted, so TCP and DNS failures for the identical target string
alone cannot meet the two-target default. TCP endpoints with different ports are
different target strings. This is temporal correlation, not proof of independent
network paths or a common cause.

At most one global correlation episode is active. Further qualifying evaluations
update its ID and last-overlap timestamp, rather than creating repeated events.
The member list is the union of affected probe identities observed during that
episode; it does not mean every member failed simultaneously. The window and
minimum-target policy stay fixed until that episode ends, including across
restarts. A new episode uses current startup configuration.

An evaluation with too few qualifying distinct targets ends the overlap. A gap
between qualifying evaluations longer than the saved window also ends the old
episode before a new one can start. `ended_at` is the ending evaluation time,
not a confirmed recovery time or an exact inferred end of network failure.
Successes and expired measurements can both end an overlap. With no new cycles,
there is no evaluation and an active event stays stored; the dashboard explicitly
warns when its last overlap is too old to describe current connectivity.
Sustained target incidents and their confirmed recovery remain separate.

Cycles older than the last processed cycle timestamp remain in raw history but
do not change correlation. Within accepted cycles, older probe outcomes do not
replace newer ones; timestamp ties follow stored TCP/DNS probe order. Keep the
host clock synchronized: large clock jumps can expire recent observations.

State and events commit atomically with snapshots, anomalies and target incidents;
a write failure rolls back the entire cycle. The detector retains at most one
outcome per identity for up to one hour (the maximum configurable window), with
limits of 128 recent identities and 128 accumulated members per episode. Exceeding
a limit produces a logged storage error rather than silently truncating a report.
Ended events follow retention based on their ending cycle and are deleted in
batches of at most 500 with their indexed membership rows. Active events and the
single bounded state record survive cleanup, even when raw history expires.

`GET /api/correlations` supports the same `range`, `to`, `type`, `target`, `limit`
and `before` parameters as the anomaly feed (default 50, maximum 200 events).
Type and target must match the same member of an event. Range selection uses
cycle timestamps and includes episodes overlapping the selected window. Responses
contain `range`, `as_of`, `count`, `correlations`, `has_more`, and `next_cursor`.
Events include `id`, `cycle_timestamp`, `detected_at`, `last_correlated_at`, nullable
`ended_at`/`end_cycle`, `members` (type and target), and `policy` (window in
nanoseconds and minimum target count). Status and membership reflect the latest
stored update even with historical `to`; this is not a reconstruction of what the
operator knew at that time. Invalid requests return JSON 400, other methods 405,
and storage failures 503. Reads have a five-second deadline and use timestamp and
member indexes with bounded cursor pages.

The shared frontend context refreshes the first page every 15 seconds and on
WebSocket reconnection. The possible shared connectivity feed appears on its
dedicated `/correlations` page. Probe filtering,
older pages, and the global time range use the existing shared pagination logic.
Loading, errors, empty selections, stale reports, affected targets and ended
overlaps have explicit states. Labels do not assert a confirmed root cause.

## Target management and scheduling

Migration 005 seeds the original two TCP and two DNS targets exactly once. Removing
all targets intentionally leaves an empty configuration across backend restarts;
defaults are not silently recreated. Target settings are independent of retention
and measurement tables, so editing or removing one never deletes its history.

Management is disabled unless the backend process has `PACKETLENS_ADMIN_TOKEN` set.
Use a randomly generated secret of 32..256 printable ASCII characters without spaces.
For example, generate a 64-character value locally using Node:

```bash
node -e "process.stdout.write(require('crypto').randomBytes(32).toString('hex'))"
```

Set the generated value in the backend process environment and restart the backend.
Do not commit it, put it in a URL, or use a `NEXT_PUBLIC_*` variable. Open `/targets`
and enter the same value to unlock management. The frontend keeps it only in
component memory, sends it as `Authorization: Bearer <token>`, and clears it on
lock/navigation/reload. It is not stored in cookies, localStorage or sessionStorage.
Use HTTPS before transmitting the token beyond local development. This is one
operator credential, not multi-user login or role-based authorization.

GET, POST, PUT and DELETE all require that token. Authentication uses a constant-time
comparison of fixed-size hashes. Browser requests must use the existing allowed
local frontend origins. CORS preflight permits Authorization and Content-Type only
on management routes; it does not add wildcard origins or cookie credentials.
A single bounded token bucket limits management requests to 30 initially, refilling
one request every two seconds, including invalid authentication attempts. Excess
requests receive JSON 429 with Retry-After. This local process limit is not a
replacement for deployment-level protection against distributed abuse.

Endpoints:

- `GET /api/targets`: `{count, targets}` for the currently applied configuration.
- `POST /api/targets`: creates a target and returns its configuration with a new ID
  (201). The body requires `type`, `address`, `enabled`, `interval_ms`, `timeout_ms`.
- `PUT /api/targets/{id}`: replaces all five editable fields (200).
- `DELETE /api/targets/{id}`: removes the target (200, `{deleted:true}`).

Use `Content-Type: application/json` for POST/PUT. Bodies are limited to 4 KiB;
unknown or missing fields, trailing JSON, malformed IDs and invalid configuration
receive JSON 400. Invalid credentials receive 401; rejected origins 403; missing
IDs 404; unsupported methods 405; duplicate type/address pairs or more than 32
targets 409; incorrect content type 415. Disabled management and database failures
return JSON 503. GET returns the in-process configuration from the last successful
save, rather than performing a database health check. Responses use `no-store`.
Each write has a five-second context deadline, and no token is logged.

Validation and limits:

- Maximum 32 targets, including disabled ones; duplicates are rejected after
  canonicalization. IDs are stable lowercase 32-character hexadecimal strings.
- TCP: a hostname or IP plus port 1..65535, with brackets around IPv6. Hostnames
  are lowercase ASCII DNS-style labels; a final dot is removed. IPs and numeric
  ports are canonicalized. Unspecified/multicast IPs, zone-qualified IPv6,
  URLs, paths and wildcard names are rejected. Private and loopback destinations
  are allowed for authorized local monitoring.
- DNS: an ASCII hostname/domain, without a scheme, port, path or wildcard. IP
  literals are rejected. Use punycode for internationalized domain names.
- Interval: 5000..3600000 milliseconds. Timeout: 100..30000 milliseconds and no
  greater than the interval. Settings apply to both TCP and DNS.

The manager serializes edits, validates the complete bounded configuration, and
replaces the SQLite rows in one transaction. Only after a successful commit does it
publish a copied configuration to the scheduler. Failed saves leave the current
configuration untouched. A process crash after commit is safe: the next startup
loads the committed configuration. Use one backend process per database; independent
writers or manual SQL edits while running are unsupported. Multiple browser tabs
can have stale editors; refresh before editing when another operator has changed
settings. The latest accepted full target replacement wins.

The existing monitoring loop now tracks due times by stable target ID, uses at most
eight workers, and gives each operation its configured context timeout. One cycle
contains the complete result set for the targets selected at dispatch. Unchanged
targets keep due times; edited or newly enabled targets become due on the next cycle.
An in-flight cycle finishes with its captured configuration, so a disabled/removed
or edited target may produce one final observation under its previous settings.
Reconfiguration never changes a partially assembled snapshot. When all targets are
disabled/removed, no empty snapshots are emitted; the dashboard can correctly show
stale measurements. Shutdown cancels probes, stops scheduling, and closes results.

Intervals are best-effort due times, not a promise of an exact sampling cadence.
A full batch of slow targets, database backpressure or a stalled runtime can delay
other due targets. Missed intervals are skipped, never replayed as catch-up bursts.
The maximum active probe concurrency remains eight regardless of configuration.
Existing incident gap policies should be adjusted when choosing longer intervals;
disabling a target does not assert recovery or close its unresolved incident.
Historical analytics/detectors still identify targets by probe type and address,
so changing an address creates a new historical identity; reusing an old address
continues its retained history and detector state. Frequent address churn also
counts toward existing analytics/correlation identity limits documented above.

The `/targets` page provides add/edit forms, explicit enable/disable controls,
removal confirmation, refresh, empty states, visible API errors, and pending states.
Only networks the operator owns or is authorized to monitor should be configured.

## DNS resolver comparison

Existing DNS monitoring still uses the system resolver and its normal LookupHost
behaviour. Optional resolver comparisons issue separate direct recursive **A-record**
queries for every enabled DNS domain, at that target's configured interval and
timeout. No resolver is queried by default. Configure process environment before
starting the backend, using endpoints you are authorized to query:

```powershell
$env:PACKETLENS_DNS_RESOLVERS = '192.0.2.53:53,[2001:db8::53]:53'
```

Those documentation addresses are placeholders; replace them with your own resolver
endpoints. The setting accepts up to three comma-separated literal IPv4/IPv6
addresses with ports 1..65535. IPv6 requires brackets. Endpoints are canonicalized;
duplicates, hostnames, unspecified/multicast addresses, missing ports and empty list
entries are rejected at startup. Empty/unset disables only comparisons. Restart the
backend to apply resolver changes. Resolver endpoints are startup configuration,
not editable through an unauthenticated query or through the target page.
New/edit/enable/disable operations on DNS targets automatically affect which domains
are compared. No new frontend environment variable is required.

Each due DNS target creates one system lookup plus one job per explicit resolver.
All jobs share the existing eight-worker pool and one complete cycle publication.
Each explicit job has one context deadline covering a UDP query and, if the response
is truncated, a TCP retry to the same endpoint. UDP queries request recursion and
use random transaction IDs. Responses must match the query ID, response flag,
operation, name, type and class. A records are accepted only for the question name
or an answer CNAME chain (at most 16 links). A CNAME-only reply without an A record
is a no-answer outcome; the probe does not run a separate chain lookup. Requests
bypass OS hosts files and search domains, and target names are absolute.

The implementation uses Go's
[`golang.org/x/net/dns/dnsmessage` codec](https://pkg.go.dev/golang.org/x/net@v0.62.0/dns/dnsmessage)
for DNS wire parsing/packing instead of a handwritten parser. This is the only added
production module. It adds no daemon or external infrastructure. No EDNS, DNSSEC
validation, DoH, DoT, AAAA queries, answer-consistency analysis, retries after ordinary
failures, or inference of resolver/network root causes is implemented. Ordinary UDP
responses are bounded to 512 bytes; TCP messages to the DNS 65535-byte length field.
The transport is conventional unencrypted DNS.

A successful comparison has a matching successful response containing an A address.
Outcomes are `success`, `timeout`, `network_error`, `dns_error` (for example NXDOMAIN),
`no_answer`, and `invalid_response`. Failed attempt durations are recorded but never
used as successful latency, and timeout counts are a subset of failed counts. Empty
and all-failed latency summaries are null. Timing includes transport and any TCP
fallback. Resolver caching, rate limits, domain mix and different sample counts can
all affect comparisons; a failure does not establish that a resolver is down.
System LookupHost may use A/AAAA, hosts files and OS-specific rules, so it is kept
separate rather than presented as an identical query benchmark.

Migration 006 adds `dns_resolver_results`, indexed by timestamp, resolver and domain.
These records commit atomically with the complete snapshot; retention deletes them
through the snapshot foreign-key cascade. Existing history and target settings are
preserved. Old system DNS records are not relabeled with an assumed explicit
resolver and no retrospective comparison data is fabricated.

Snapshot REST/WebSocket payloads gain an optional `dns_comparisons` array. Its rows
include the existing probe fields plus `resolver` and `outcome`. The `dns` array and
existing metrics/stats/analytics/availability, anomaly, incident and correlation
semantics continue to describe the system lookup. Explicit resolver results do not
advance those detectors or inflate their counts. Snapshot hydration and frontend
fallback identities preserve the added comparison data across reloads and time
range changes. At most 96 comparison rows can occur in a cycle (32 domains times
three endpoints).

`GET /api/dns/resolvers?range=15m` accepts the existing `range` and optional `to`
parameters, with the same validation as persisted analytics. It does not trigger
probes or accept arbitrary resolver addresses. Responses contain `range`, `as_of`,
`from`, `to`, `configured_resolvers`, and summaries in `groups` (per endpoint) and
`targets` (per domain/endpoint). Summaries include attempts, successful/failed counts,
timeouts, mean/min/max successful latency, most recent failure timestamp/error, and
60 time buckets spanning the whole selection. Groups weight successful samples,
not domain means. Empty buckets have null latency and availability; observed
all-failed buckets have 0% availability and null latency. Reports stream database
rows into bounded summaries, with at most 128 historical domain/endpoint identities,
a 30-second deadline and the existing two-request analytics concurrency limit.
Bad queries return JSON 400, other methods 405, busy analytics 429, and storage
failures 503. All existing global ranges, including All retained data, are supported.

The dedicated `/resolvers` page displays the shared persisted report, refreshes every 15
seconds and on WebSocket reconnection, and follows the global time range. Its domain
selector, outcome table, resolver latency lines and last-failure details include
loading, error, stale and empty states. Historical endpoints remain visible even
if removed from current startup configuration. Comparing one domain provides a more
comparable sample mix than the all-domain mean. Configuration changes do not erase
retained results.

## CSV export and reporting

Open **Exports** in the navigation. Choose Measurements, DNS
resolver comparisons, or Incident history; optionally select TCP/DNS and enter an
exact target. Suggestions come from the loaded analytics, but any retained target
can be entered even if it is no longer configured. Resolver exports use DNS only.
The global time range, filters and UTC end time are captured when Download CSV is
clicked. Later time-range changes do not change an in-progress download. Empty
selections produce a valid header-only file. Cancel stops the pending request;
API errors and interrupted transfers remain visible without saving a partial file.

The three GET endpoints accept only:

| Parameter | Meaning |
| --- | --- |
| `range` | `5m`, `15m` (default), `1h`, `6h`, `24h`, or `all` retained data |
| `to` | Optional inclusive RFC3339 end time, default server request time |
| `type` | Optional `tcp` or `dns`; resolver exports reject `tcp` |
| `target` | Optional exact target string |

Measurement and resolver selection uses inclusive **snapshot** timestamps, matching
the global range convention. Probe timestamps are exported separately. `all` has no
lower bound and includes all matching retained rows through `to`; it does not mean
unlimited permanent retention. Unknown, repeated, empty or malformed parameters
receive JSON 400. Exports do not accept history pagination parameters or a `from`
parameter. Use a finite `range` with a fixed `to` to split large selections into
smaller periods (boundaries are inclusive, so account for a boundary when combining
adjacent exports). POST/other methods return JSON 405.

- `/api/export/measurements.csv`: `timestamp`, `snapshot_timestamp`, `target`,
  `probe_type`, `latency_ms`, `success`, `error`. Includes TCP and system DNS with
  the same semantics as existing metrics. Rows follow probe time, then stable
  cycle/ID/ordinal ordering. Failed latency values are recorded attempt durations,
  not successful resolution/connectivity timings; use the success column.
- `/api/export/dns-resolvers.csv`: the same columns plus `resolver` and `outcome`.
  Includes only explicit resolver comparisons, ordered by cycle, ID and ordinal.
  Resolver identity, timeouts and other failure categories survive export.
- `/api/export/incidents.csv`: `id`, `probe_type`, `target`, `status`, `started_at`,
  `cycle_timestamp`, `detected_at`, `last_failure_at`, `last_observed_at`,
  `recovered_at`, `elapsed_span_ms`, `failed_probes`, `recovery_successes`,
  `has_monitoring_gaps`, `first_error`, `last_error`, `failure_threshold`,
  `recovery_threshold`, `max_gap_ms`. Rows follow first-failure cycle and ID order.

Incident selection includes episodes overlapping the time window, including older
open episodes. Status and recovery metadata are the latest stored state when the
query starts, even with historical `to`; this is not an immutable audit log of what
was known then. Open episodes have empty recovery/duration fields; a recovered
zero-duration span exports numeric zero. Elapsed spans can include unknown gaps and
do not prove continuous downtime. Correlation/anomaly episodes are not included in
incident exports.

Files use UTF-8 CSV with a header, comma delimiters and LF record endings. Go's
`encoding/csv` quotes embedded commas, quotes and newlines. Text fields, including
errors, retain their literal values. Timestamps are UTC RFC3339Nano and latency is
milliseconds; booleans are `true`/`false`, with no fake production rows or summary
footer. These files can be analyzed with a CSV-aware reader rather than splitting
text on commas or line breaks.

SQLite walks one row at a time; neither raw measurement history nor the full CSV
is loaded into Go memory. Incident exports use a single ordered read rather than
repeated feed pages/count queries. Each dataset is read with one consistent SQL
statement, so rows committed after that read begins are not mixed into it. Retention
and new monitoring writes can continue in WAL mode. Long reads can temporarily
hold WAL pages; the deadlines below bound their lifetime. No migration or production
dependency was added for export.

A backend process permits one export at a time, separately from the two concurrent
analytics reads, leaving a database connection available for collection. A bounded
request bucket permits six accepted export attempts initially and refills one every
ten seconds. Busy/rate-limited requests receive JSON 429 with Retry-After; ordinary
storage failures before output begins return generic JSON 503. Each export has a
60-second context deadline and a five-second write deadline refreshed during
stream flushes, with output flushed at least every 128 rows. Client cancellation
stops database iteration. If an error occurs after output has begun, the server
aborts the HTTP stream using Go's designated abort mechanism; it does not append
JSON to CSV or silently finish a truncated file. Failures are logged with dataset
and emitted-row context. Discard any incomplete output from a failed external
transfer.

The dashboard reads the response incrementally and saves a Blob only after the
stream completes successfully. It caps each browser download at **32 MiB** and
shows received bytes, pending/cancel states, failures, and completion initiation.
This limit prevents an unbounded JavaScript buffer. For a larger download, use the
captured **Export URL for large files** with a tool that streams directly to disk,
for example:

```bash
curl --fail --output measurements.csv "http://localhost:8080/api/export/measurements.csv?range=all&type=tcp"
```

The backend's 60-second deadline still applies; reduce the range or filter by target
if needed. Export is read-only and uses the same public monitoring-data access as
history APIs, rather than the target-management admin token. Existing origin rules
remain unchanged. Remote deployments still need the access controls described in
Run locally. There is no email delivery, scheduled reporting, PDF report generation,
or automatic aggregation in CSV exports.

## Checks

From `backend/`:

```bash
go test ./...
go vet ./...
```

Format changed Go files with `gofmt -w <files>`. Backend regression tests cover
HTTP routes/CORS/statistics, store retention and isolation, concurrent store
access, slow clients/shutdown, and local or cancelled probes. SQLite tests also
cover restart persistence, migrations, atomic writes, cursor pagination, time
boundaries, retention cascades, concurrent access, cancellation, database errors,
and configuration validation. Analytics tests cover nearest ranks, histogram
boundaries, target isolation, jitter chronology/failure gaps, period boundaries,
availability windows and streaks, large histories, and concurrent reads/writes.
Anomaly tests cover warmup, zero/nonzero MAD, strict thresholds, failure exclusion,
episode recovery, restart continuity, atomic rollback, migration from schema v1,
cursor/filter behaviour, marker counts, and retention of unresolved episodes.
Incident tests cover isolated failures, failure/recovery thresholds, gaps, frozen
policy, restart continuity, atomic rollback, migration from v2, identity isolation,
pagination/count/status contracts, invalid state, and recovered/open retention.
Correlation tests cover inclusive windows, distinct target names, superseding
successes, expired observations, monitoring gaps, policy retention, out-of-order
cycles, limits, concurrent writes, restart continuity, rollback, migration from v3,
filtered cursor pages, API validation, and retention cascades. DNS comparison tests use local UDP/TCP servers for successful replies, truncation
fallback, malformed/mismatched packets, NXDOMAIN, missing/unrelated A answers,
timeouts and cancellation. Storage/report tests cover resolver identity, rollback,
restart hydration, migration from v5, original-stat isolation, retention, ranges,
trend counts and all-failed/null latency. Export tests cover literal CSV escaping, inclusive bounds/type/target filters,
header-only files, resolver fields, incident overlap/status/duration, cancellation,
consumer errors, more than 1000 persisted cycles, early and late storage failures,
real interrupted HTTP transfers, and request/concurrency limits. Tests do not need
public internet access. Target tests cover canonical validation, duplicates/limits,
serialized concurrent edits, persistence/empty restarts, migration from v4, write
rollback, API authentication/origins/body/method/rate limits, bounded probe workers,
per-target due times, in-flight reconfiguration, timeouts and cancellation.

From `frontend/`:

```bash
npm test
npm run lint
npx tsc --noEmit
npm run build
```

Frontend tests run through tsx and Node's test runner. jsdom supplies a simulated
DOM for React component tests. Coverage includes nested response validation,
REST/live merging, time ranges, empty/failed states, formatting, target/window
selectors, abort cleanup, API errors, and late-response protection. Anomaly tests
cover response validation, severity/recovery rendering, feed filtering/pagination,
and visible onset markers. Incident tests verify lifecycle validation/rendering,
zero-duration recovery, gap warnings, API errors, status/probe filters, pagination,
and range changes while viewing older pages. Correlation tests cover payload
validation, affected targets, uncertainty warnings, ended overlaps, filters,
pagination, range resets, errors and empty reports. Target tests cover authentication, add/edit/toggle/removal
interactions, private request headers, API errors, empty configuration, token
locking and response validation. Resolver tests cover report/snapshot validation,
historical identity, latency/failure displays, domain/range selection, empty states
and API failures. Export tests cover URL encoding, completed streamed responses,
browser size limits, cancellation, interrupted transfers, dataset/filter/range
controls, API failures and object-URL cleanup. DOM chart
containers have deterministic test dimensions; these tests do not replace visual
browser verification. The added tools are devDependencies only.

## Remaining limitations

- Recent raw activity/chart sections remain limited to 1,000 cached cycles.
  Persisted summaries and new trend charts cover the full selected retained range.
- Local origins and HTTP listen address remain configured in source code.
- Target intervals are best-effort; slow batches can delay subsequent due probes.
- Browser CSV downloads are capped at 32 MiB; server exports have a 60-second deadline.
- Deleted history and cycles that failed persistence cannot be recovered.
- Retention is based on age, not a disk-space quota; operators must provision disk.
- Exact percentile sorting can be expensive on very large histories. Queries
  use bounded concurrency and deadlines; larger deployments may need rollups.
- Trend buckets average observations and can hide short transient extremes.
- Availability describes observed probes, not verified continuous service uptime.
- Incident durations are elapsed observation spans; they can include unknown gaps.
- Cross-target correlation describes temporal overlap, not a confirmed cause or
  verified shared infrastructure; missing measurements can end the overlap.
- Stale status appears after 15 seconds without a snapshot. It is a dashboard
  diagnostic and does not establish that the monitored network is down.
- Admin token authentication protects target management only. Public monitoring
  APIs and CSV exports have no user authentication. A full browser regression
  suite remains pending. Avoid public exposure until deployment controls are added.

Keep changes focused and tested. Read `AGENTS.md`, trace both sides of any API
contract, and preserve the distinction between failed probes and valid latency.

## dashboard layout and browser checks

The overview contains summary cards and TCP/DNS latency trends. Other data has
its own page: `/tcp`, `/dns`, `/jitter`, `/resolvers`, `/health` (availability),
`/anomalies`, `/incidents`, `/correlations`, `/activity`, `/targets`, and `/exports`.
TCP/DNS data tabs isolate latency, percentiles, distribution, jitter, targets and
period comparisons. The activity page separates primary probes, resolver queries
and cached target trends. Global filters and shared monitoring state survive
client-side navigation. Availability still uses its clearly labeled fixed windows.

Charts share time labels, axes and tooltip styling. Monotone lines soften the
rendering without changing measurements or joining nulls. Sparse series retain
visible point markers. Inactive data panels are not rendered; tab controls support
arrow keys, Home/End, visible focus, and accessible selected/panel relationships.
The mobile page selector uses the same route registry as the desktop navigation.

`npm run typecheck` checks TypeScript. `npm run test:e2e` runs the Playwright browser
suite against a local Next server with test-only monitoring responses. Run
`npx playwright install chromium` once, or set `PLAYWRIGHT_CHANNEL=msedge` to use
an installed Microsoft Edge. Browser tests cover all navigation destinations,
data-tab keyboard controls, curve rendering and narrow-screen overflow. Generated
screenshots, traces and reports stay in ignored test artifact directories. No
public probes or production data changes are required. The lint parser's minimatch
override keeps its glob matcher on the patched 11.x version.
