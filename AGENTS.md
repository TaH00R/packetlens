# AGENTS.md --- PacketLens

Repository-level instructions for AI coding agents.

## Project purpose

PacketLens is a network observability dashboard. It periodically runs
TCP-connectivity and DNS probes, groups measurements into monitoring
snapshots, stores them, streams live updates, and visualizes latency and
network health.

Priorities: correctness, maintainability, clear architecture, useful
diagnostics, and a polished dashboard. Do not add complexity or
dependencies without a concrete reason.

## Stack

-   Backend: Go
-   Frontend: Next.js App Router, React, TypeScript
-   Styling: Tailwind CSS
-   Charts: Recharts
-   Icons: Lucide React
-   Realtime: WebSocket
-   Current storage: bounded in-memory `MemoryStore`
-   Probe types: TCP and DNS

Inspect `go.mod`, `package.json`, configuration, and the actual
repository structure before making assumptions or adding dependencies.

## How to work in this repository

1.  Inspect relevant files and trace the existing data flow before
    editing.
2.  Extend existing implementations rather than creating parallel
    systems.
3.  Make small, complete, testable changes. Do not implement the entire
    roadmap in one unreviewed change.
4.  Keep modules focused; avoid giant handlers/components, duplicated
    logic, and unnecessary abstractions.
5.  Use explicit types. Avoid TypeScript `any`.
6.  Handle loading, empty, error, disconnected, reconnecting, and
    stale-data states.
7.  Do not silently swallow errors. Log useful context without logging
    secrets.
8.  Do not invent endpoints, fields, environment variables, or existing
    behaviour. Verify them in code.
9.  Avoid destructive commands and preserve unrelated user changes.
10. After changes, inspect the diff and report changed files, checks
    run, and remaining caveats.

If a requirement is ambiguous and a wrong choice would cause significant
rework, ask one focused question. Otherwise make a sensible, reversible
assumption and state it.

## Current architecture and data flow

The repository has two main applications:

``` text
packetlens/
├── backend/
└── frontend/
```

### Backend

Known package responsibilities:

-   `backend/internal/probe`: TCP and DNS probe logic.
-   `backend/internal/monitor`: monitoring orchestration and
    result/snapshot types.
-   `backend/internal/store`: storage implementation, currently
    including `MemoryStore`.
-   `backend/internal/realtime`: WebSocket hub and client broadcasting.
-   `backend/internal/api`: HTTP and WebSocket handlers.
-   `backend/main.go` and server setup: application wiring, collection
    loop, and lifecycle. Inspect the actual files to confirm exact
    responsibilities.

The core concepts are:

-   `ProbeResult`: one measurement, including type, target, latency,
    success/failure, error, and timestamp.
-   `Snapshot`: one complete monitoring cycle, with a timestamp and
    TCP/DNS result collections.

The frontend currently consumes snapshot fields `timestamp`, `tcp`, and
`dns`. Probe result fields include `type`, `target`, `latency_ms`,
`success`, optional `error`, and `timestamp`.

**Always verify actual Go JSON tags and TypeScript definitions before
relying on these names.** Keep the API contract aligned on both sides.

### Known HTTP routes

The server has been wired with:

-   `GET /api/health`
-   `GET /api/metrics`
-   `GET /api/stats`
-   `GET /api/snapshots`
-   `/api/ws` for WebSocket streaming

Current response concepts:

-   Health returns service status and timestamp.
-   Metrics returns flattened probe results and a count.
-   Stats returns aggregate statistics grouped by probe type.
-   Snapshots returns a count and complete snapshots.
-   WebSocket streams monitoring snapshots.

Inspect route registration and handlers before changing response shapes.
If an API contract changes, update all consumers and tests in the same
task.

### Frontend

Known frontend areas:

-   `frontend/src/types/metrics.ts`: shared monitoring types.
-   `frontend/src/hooks/usePacketLensSocket.ts`: WebSocket lifecycle,
    reconnect, and live snapshot state.
-   `frontend/src/context/MonitoringContext.tsx`: shared monitoring
    state and derived history.
-   `frontend/src/components/layout/DashboardShell.tsx`: shared
    dashboard shell.
-   `frontend/src/components/dashboard/`: navigation, header, selectors,
    reusable dashboard UI.
-   `frontend/src/components/`: charts and reusable UI.
-   `frontend/src/app/`: App Router pages, including overview and
    analytics pages.

Treat this map as a guide; inspect actual paths and exports before
editing.

## Frontend rules

### React and Next.js

-   Use Client Components only when hooks, browser APIs, event handlers,
    or client-side state require them.
-   Keep server/client boundaries intentional; do not add `"use client"`
    everywhere.
-   Prefer shared context for monitoring data instead of fetching
    independently on each page.
-   Clean up WebSockets, listeners, and subscriptions.
-   Prevent stale closures and state updates after unmount.
-   Use `useMemo` when it improves clarity or avoids meaningful repeated
    work, not automatically.

### Monitoring state

The shared monitoring context should be the single frontend source of
truth for:

-   connection status
-   latest snapshot
-   all available history
-   selected time range
-   filtered history
-   reconnect action

Keep **all available history** separate from **filtered history**.
Changing the selected range must never delete or mutate stored
measurements.

When merging REST history and WebSocket data:

-   Deduplicate with a stable identity.
-   A timestamp is acceptable only if the backend guarantees uniqueness;
    otherwise introduce a stable snapshot ID.
-   Sort chronologically.
-   Ensure a late REST response cannot overwrite newer live data.
-   A REST failure must not break live monitoring.
-   Do not assume in-memory history survives a backend restart.

Treat HTTP and WebSocket payloads as untrusted input. Validate their
shape before using them.

### Time ranges

The intended global ranges are:

-   Last 5 minutes
-   Last 15 minutes
-   Last hour
-   Last 6 hours
-   Last 24 hours
-   All available data

Apply filtering consistently across overview, TCP, DNS, and
network-health pages. Keep the selector in the shared dashboard shell.

Use snapshot timestamps as the time basis. Handle empty data and
boundary conditions. "All" means all data currently available from the
backend, not permanent history. Consider how the visible range advances
as time passes, even when no new snapshot arrives; clean up any timer
introduced.

### Charts and metrics

-   Use actual measurements. Never fabricate production chart data.
-   Latency units are milliseconds.
-   Never represent failed probes as successful zero-millisecond
    measurements. Use gaps/nulls or a separate failure visualization.
-   TCP connection time is not ICMP round-trip time.
-   DNS timing is affected by resolver behaviour and caching.
-   Probe success rate is not packet loss.
-   Define jitter clearly and calculate it from ordered, comparable
    measurements.
-   Handle empty data, failed probes, missing targets, and long time
    ranges.
-   Do not truncate a selected time range to the latest 60 points
    without an explicit, justified downsampling strategy.
-   Keep axes, tooltips, legends, units, and responsive chart containers
    readable.
-   Avoid excessive animation on frequently updating charts.

### Visual direction

The existing dashboard uses a dark network-observability style:

-   Main background: approximately `#0B1016`
-   Panel background: approximately `#111820`
-   Subtle borders: approximately `white/[0.07]`
-   Primary accent: teal, e.g. `#14B8A6`
-   Secondary accent: blue, e.g. `#3B82F6`
-   Muted text must remain legible

Keep pages visually consistent. Prefer reusable cards, tables, badges,
and chart components. Ensure responsive layouts and avoid horizontal
overflow. Do not redesign the whole application for a focused feature.

## Backend rules

### Go style and concurrency

-   Run `gofmt` on changed Go files.
-   Keep packages cohesive and avoid circular dependencies.
-   Wrap errors with useful context.
-   Use `context.Context` for cancellation and timeouts where
    appropriate.
-   Avoid panics for expected runtime failures.
-   Validate HTTP methods and request parameters.
-   Use explicit status codes and consistent JSON responses.
-   Avoid unbounded goroutines and channels.
-   Protect shared mutable state correctly.
-   Do not hold locks during network I/O.
-   Give background goroutines a clear shutdown path.

### Probes and scheduler

-   Use explicit timeouts for TCP and DNS probes.
-   Record timestamps consistently.
-   Distinguish failure from high latency.
-   Preserve useful error details without exposing excessive low-level
    noise to users.
-   One slow target must not block an entire monitoring cycle
    indefinitely; use bounded concurrency and per-probe timeouts.
-   A snapshot should represent one complete monitoring cycle.
-   Do not publish partially assembled snapshots unless partial-cycle
    semantics are deliberately designed.
-   Slow WebSocket clients must not stall the collector. Use an explicit
    drop/disconnect policy and make it observable.
-   Ensure the collector, HTTP server, and WebSocket hub shut down
    cleanly.

### Storage

`MemoryStore` is process-local and bounded. It is suitable for
development, not durable history.

When adding persistence:

1.  Inspect the existing store API and callers.
2.  Define retention and query requirements.
3.  Keep storage behind a small interface.
4.  Add migrations/schema versioning where appropriate.
5.  Index timestamp and frequently filtered dimensions.
6.  Use transactions for related writes.
7.  Handle storage errors explicitly.
8.  Avoid loading unlimited history into memory; use time-bounded
    queries or pagination.
9.  Preserve a straightforward local development setup.

SQLite is a reasonable first durable-storage option for a
single-user/local deployment, but verify requirements before choosing
it. Do not assume a remote database is necessary.

## Security, privacy, and operational safety

-   Use PacketLens only for networks and systems the operator owns or is
    authorized to monitor.
-   Do not add credential capture, traffic decryption, stealth,
    persistence, intrusive scanning, or interception features.
-   Keep probes limited to configured targets and reasonable rates.
-   Do not expose arbitrary host/port probing to unauthenticated remote
    users.
-   Validate configurable targets and enforce sensible limits.
-   Keep secrets in environment variables; never commit secret-bearing
    `.env` files.
-   Use `.env.example` with safe placeholders where useful.
-   Restrict CORS and WebSocket origins to intended frontend origins. Do
    not use wildcard origins with credentials.
-   Avoid exposing stack traces or sensitive system details in API
    responses.
-   Add authentication/rate limiting if remotely accessible
    administrative or configuration endpoints are introduced.
-   Use TLS/WSS in deployed environments.
-   Describe the scope and limitations of configured probes; do not
    claim they prove an entire network is healthy.

## Configuration

The frontend currently expects these local-development variables:

``` env
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_WS_URL=ws://localhost:8080/api/ws
```

Inspect existing configuration before adding variables. Restart the
Next.js development server after changing `.env.local`.

These public URL variables are not secrets. Never expose private backend
secrets through `NEXT_PUBLIC_*`.

Backend configuration should be documented, validated at startup, and
have sensible local defaults where appropriate.

## Testing and verification

Add tests alongside new behaviour.

### Backend tests

Where relevant, cover:

-   TCP success, timeout, and connection failure.
-   DNS success, timeout, and lookup failure.
-   Snapshot assembly and scheduler cancellation.
-   Store insertion, ordering, capacity/retention, and concurrent
    access.
-   HTTP method handling and JSON response contracts.
-   Stats for empty, all-success, all-failure, and mixed results.
-   WebSocket registration, broadcasting, slow clients, and shutdown.
-   Persistence queries and migrations.

Prefer local test servers, fake resolvers, injected clocks, and
interfaces. Unit tests should not depend on public internet
availability.

### Frontend tests

Where relevant, cover:

-   Snapshot validation.
-   REST history loading and failure handling.
-   REST/WebSocket merge and deduplication.
-   Time-range boundaries.
-   Empty and failed-probe chart states.
-   Metric calculations.
-   Reconnect and cleanup behaviour.

### Commands

Run checks appropriate to the changed code:

Backend:

``` bash
gofmt -w <changed-go-files>
go test ./...
go vet ./...
```

Frontend, from `frontend/`:

``` bash
npm run lint
npx tsc --noEmit
npm run build
```

Check available scripts before assuming they exist. If a command cannot
run, report why. Never claim a check passed unless it was actually run.

## Suggested roadmap

This is a prioritization guide, not a request to implement everything at
once. Inspect the repository and avoid duplicating completed work.

### Phase A --- Stabilize current functionality

-   [ ] Verify health, metrics, stats, snapshots, and WebSocket
    endpoints.
-   [ ] Verify REST historical hydration.
-   [ ] Verify live WebSocket updates and reconnect behaviour.
-   [ ] Fix stable snapshot identity and REST/live merge edge cases.
-   [ ] Verify global time-range filtering across every analytics page.
-   [ ] Remove chart-side truncation that contradicts selected ranges.
-   [ ] Add loading, empty, stale, disconnected, and API-error states.
-   [ ] Check responsive layout and accessibility.

### Phase B --- Improve analytics

-   [ ] Improve TCP and DNS target breakdowns.
-   [ ] Add useful failure-history and failure-rate visualizations.
-   [ ] Define and test jitter calculations.
-   [ ] Add availability summaries with clearly defined windows.
-   [ ] Add target-level health indicators.
-   [ ] Add deliberate chart downsampling if performance requires it.
-   [ ] Make units, time zones, and metric definitions explicit.

### Phase C --- Durable storage

-   [ ] Define a storage interface around current store operations.
-   [ ] Choose and document a local persistence solution; consider
    SQLite.
-   [ ] Add schema migrations.
-   [ ] Persist snapshots and probe results.
-   [ ] Query by time range instead of loading unlimited history.
-   [ ] Add retention/cleanup policy and tests.
-   [ ] Handle storage outages gracefully.

### Phase D --- Configuration and operations

-   [ ] Make probe targets and intervals configurable.
-   [ ] Validate configuration at startup.
-   [ ] Add structured logging.
-   [ ] Add backend operational metrics.
-   [ ] Add readiness/liveness semantics if needed.
-   [ ] Document local development and deployment.
-   [ ] Add Docker support only after configuration is stable.

### Phase E --- Product polish

-   [ ] Improve onboarding and explain metric meanings and limitations.
-   [ ] Add safe target management only with validation and access
    controls.
-   [ ] Add export of selected measurements, such as CSV, if useful.
-   [ ] Improve dashboard connection and error diagnostics.
-   [ ] Add end-to-end tests for the main monitoring flow.
-   [ ] Prepare screenshots and a clear README.

## Definition of done

A feature is complete when:

-   Its behaviour matches the requirement.
-   Existing API contracts and pages continue to work.
-   Loading, empty, error, and boundary cases are handled.
-   Types, validation, and tests are updated.
-   Relevant checks have actually been run.
-   No fake data, dead controls, or unexplained placeholders remain.
-   Configuration and documentation are updated where necessary.
-   The final report lists changed files, checks run, and known
    limitations.

## Required agent workflow

For each task:

1.  **Understand:** inspect the relevant code and identify current
    behaviour.
2.  **Plan:** state a short implementation plan and likely files to
    change.
3.  **Implement:** make the smallest complete change consistent with
    this document.
4.  **Verify:** run focused tests, then broader checks where practical.
5.  **Review:** inspect the diff for regressions, accidental churn,
    secrets, and unrelated edits.
6.  **Report:** summarize behaviour, changed files, checks, and
    remaining work.

Keep the application runnable after each milestone. Do not attempt the
entire roadmap in one unreviewed change.
