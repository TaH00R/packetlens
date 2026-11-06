# PacketLens

PacketLens is a local network observability dashboard built with Go, Next.js,
React, and TypeScript. It runs TCP connection and DNS lookup probes, stores
complete monitoring cycles in SQLite, and streams snapshots to the dashboard.
Use it only for destinations you own or are authorized to monitor.

## Features

- Overview, TCP Analytics, DNS Analytics, Network Health, and a metrics guide.
- Average successful latency, success/failure counts, and per-target health.
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
those values. Frontend latency cards show a dash when no successes exist.

## Persistent history and bounded queries

SQLite stores snapshots and individual probe results with indexed timestamps,
targets, and types. Embedded numbered SQL migrations run transactionally at
startup, with schema version recorded in `PRAGMA user_version`. WAL mode,
foreign keys, a five-second busy timeout, and full synchronous commits are enabled.
The local connection pool is limited to one connection to serialize database work.
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

## Dashboard history and time ranges

The browser still retains up to 1,000 merged snapshots to bound memory use,
independently of backend retention. At a five-second cadence this is roughly
83 minutes. This storage milestone does not yet make the dashboard fetch older
pages for 6-hour, 24-hour, or All selections; those views may be incomplete.
The selector explicitly notes this cache limit. Older retained data is available
through the paginated API above. Range-aware UI loading is a later milestone.

The browser validates payloads, deduplicates by stable snapshot ID (falling back
to timestamp and probe contents for legacy/live-only samples), and sorts cycles
chronologically. A late REST response cannot discard newer live samples.
Reconnection reloads the latest retained backend page.

Changing the selected range filters snapshots without deleting stored history.
All analytics pages share the range. Filtering uses inclusive snapshot timestamp
boundaries; a five-second timer advances the window even without new data.
Charts display every cached snapshot in the range, with no separate 60-point
limit. Times are displayed in the browser's local time zone. `All` currently means
all cached browser history, not all retained SQLite records.

## Metric definitions

All latency and jitter values use milliseconds.

- **TCP latency:** elapsed time to establish a TCP connection. It is not ICMP
  ping or a direct measurement of network round-trip time.
- **DNS latency:** elapsed time for name resolution using the backend machine's
  resolver. Resolver selection and caching affect the result.
- **Average latency:** sum of successful latencies divided by successful count.
  For 20, 25, and 30 ms, the average is 25 ms.
- **Minimum / maximum:** lowest / highest successful latency in the selection.
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
and configuration validation. Tests do not need
public internet access.

From `frontend/`:

```bash
npm test
npm run lint
npx tsc --noEmit
npm run build
```

Frontend regression tests cover nested payload validation, REST/live merging,
identity collisions, retention, and time-range boundaries using Node's built-in
test runner. They do not replace browser interaction tests.

## Remaining limitations

- The dashboard cache remains limited to 1,000 cycles; full persistent-range UI
  loading is still a later milestone. SQLite is durable, subject to retention.
- Targets, probe interval, and local origins remain configured in source code.
- Deleted history and cycles that failed persistence cannot be recovered.
- Retention is based on age, not a disk-space quota; operators must provision disk.
- Database work uses one local connection; long queries can delay other operations
  until the request deadline. Larger deployments will need measured query tuning.
- Stale status appears after 15 seconds without a snapshot. It is a dashboard
  diagnostic and does not establish that the monitored network is down.
- There is no authentication, target-management UI, CSV export, or full browser
  regression suite. Avoid public exposure until deployment controls are added.

Keep changes focused and tested. Read `AGENTS.md`, trace both sides of any API
contract, and preserve the distinction between failed probes and valid latency.
