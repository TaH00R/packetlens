# PacketLens

PacketLens is a local network observability dashboard built with Go, Next.js,
React, and TypeScript. It runs TCP connection and DNS lookup probes, stores
complete monitoring cycles in SQLite, and streams snapshots to the dashboard.
Use it only for destinations you own or are authorized to monitor.

## Features

- Overview, TCP Analytics, DNS Analytics, Network Health, and a metrics guide.
- Exact P50/P90/P95/P99 successful latencies, distributions, target comparisons, and prior-period comparisons.
- Per-target mean and latest-pair jitter, with historical trends.
- Persistent rolling median/MAD anomaly episodes, duplicate suppression, a feed, and chart markers.
- Observed availability for 1h, 24h, 7d, and 30d, with outcome streaks and timestamps.
- Aggregate and per-target latency charts; failures appear as gaps.
- Global ranges: last 5 minutes, 15 minutes, hour, 6 hours, 24 hours, or All.
- Durable SQLite snapshots and probe results, with migrations and configurable retention.
- Bounded REST history pages with time/type/target filters and cursor pagination.
- REST history hydration at startup and on WebSocket connection/reconnection.
- Validated payloads, deduplication, chronological merging, and automatic reconnects
  with a delay from 1 to 15 seconds.
- Loading, history-error, connection, empty, and stale measurement messages.
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
|   |   |-- api/          # HTTP and WebSocket handlers
|   |   |-- monitor/      # Complete monitoring cycles and types
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
configuration, HTTPS/WSS, and access controls; do not expose the unauthenticated
API directly to the public internet.

## Monitoring and configuration

`backend/cmd/packetlens/main.go` currently configures:

| Setting | Current value |
| --- | --- |
| Monitoring interval | 5 seconds |
| TCP targets | `1.1.1.1:443`, `8.8.8.8:443` |
| DNS domains | `google.com`, `cloudflare.com` |
| Backend storage | SQLite at `data/packetlens.db` |
| History retention | 720 hours (30 days) |
| Cleanup interval | 1 hour, including a sweep at startup |
| Default history page | Latest 1,000 cycles |
| HTTP address | `:8080` (in `internal/api/server.go`) |

Targets, probe interval, and HTTP address still require code changes and a restart.
Both probe types use a 3-second timeout and respect parent cancellation.

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

A cycle waits for all its probes before storing and broadcasting a snapshot.
The snapshot and its probe rows are committed together in one transaction.
Stored snapshots have an additive `id` field shared by REST and live messages.
The collector runs immediately at startup and then on the interval. Slow
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
| GET | `/api/health` | `status`, `service`, `timestamp` |
| GET | `/api/metrics` | `count`, `results` (flattened probes) |
| GET | `/api/stats` | `total_results`, `probes` (statistics by type) |
| GET | `/api/analytics` | Full-range latency, jitter, histogram, trends, previous period |
| GET | `/api/anomalies` | Bounded, filtered anomaly episode feed |
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
reconnection. Dashboard, TCP, DNS, and health pages provide a chronological feed,
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
in this milestone, and failure-incident detection remains a separate next milestone.

## Dashboard history and time ranges

The shared monitoring context loads persisted analytics and anomalies every 15 seconds
and availability every minute, also refreshing on WebSocket connection/reconnection.
All pages consume that shared state. Overview summaries and latency trends,
TCP/DNS analytics, and health jitter use all retained measurements in the global
selection. All means all retained SQLite history for these reports. Availability
uses its explicitly labeled fixed windows independently of the global range.

Target selection controls percentile cards, distributions, and latency/jitter
charts. Comparison tables show all targets in the selected probe type. The UI
shows observation counts, timestamps, partial-history notes, loading/API errors,
and stale-report warnings (45 seconds for analytics, two minutes for availability).
Changing ranges aborts the old request and clears the old range's report until the
new one arrives; a late old response cannot overwrite the selection.

Recent activity and the overview's recent target chart still use a browser cache
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
Tests do not need public internet access.

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
and visible onset markers. DOM chart
containers have deterministic test dimensions; these tests do not replace visual
browser verification. The added tools are devDependencies only.

## Remaining limitations

- Recent raw activity/chart sections remain limited to 1,000 cached cycles.
  Persisted summaries and new trend charts cover the full selected retained range.
- Targets, probe interval, and local origins remain configured in source code.
- Deleted history and cycles that failed persistence cannot be recovered.
- Retention is based on age, not a disk-space quota; operators must provision disk.
- Exact percentile sorting can be expensive on very large histories. Queries
  use bounded concurrency and deadlines; larger deployments may need rollups.
- Trend buckets average observations and can hide short transient extremes.
- Availability describes observed probes, not verified continuous service uptime.
- Stale status appears after 15 seconds without a snapshot. It is a dashboard
  diagnostic and does not establish that the monitored network is down.
- There is no authentication, target-management UI, CSV export, or full browser
  regression suite. Avoid public exposure until deployment controls are added.

Keep changes focused and tested. Read `AGENTS.md`, trace both sides of any API
contract, and preserve the distinction between failed probes and valid latency.
