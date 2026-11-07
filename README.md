# packetlens

a small network observability dashboard built with go, next.js and sqlite.
it runs tcp and dns checks, keeps their history, and streams new measurements
to the browser.

## what it does

- latency, percentiles, distributions and period comparisons
- jitter and observed availability per target
- latency anomalies, sustained incidents and overlapping failures
- dns resolver comparisons
- target management with an admin token
- filtered csv exports

each kind of data has its own page. tcp and dns analytics have separate tabs
for latency, percentiles, distributions, jitter, targets and comparisons.

## run it

requires go 1.27.1+, node 24.15+ and npm.

```sh
cd backend
go mod download
go run ./cmd/packetlens
```

in another terminal:

```sh
cd frontend
npm ci
npm run dev
```

open http://localhost:3000. the backend listens on port 8080.
sqlite migrations run at startup; saved history and target settings survive restarts.
target management is locked until you configure an admin token.

## checks

```sh
cd backend
go test -race ./...
go vet ./...
```

```sh
cd frontend
npm test
npm run lint
npm run typecheck
npm run build
npx playwright install chromium
npm run test:e2e
```

browser tests use local fixtures and do not need public dns or tcp access.

## a few limits

availability describes observed probes, not uninterrupted uptime. missing data
stays unknown, and correlated failures do not prove a shared cause. live activity
keeps 1,000 cached cycles; stored analytics use the selected retained history.
only configure systems you are authorized to monitor. this is a local-first app;
public deployment needs tls, origin restrictions and authentication for read apis.

see the [setup and api reference](docs/reference.md) for environment variables,
metric definitions, retention, resolver setup and export limits.
