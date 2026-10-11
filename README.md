# AdPod Lab: Intelligent CTV Ad-Pod Optimization
### by Raymond Gan

AdPod Lab is a Go-based CTV ad-decisioning simulation. It resolves creative identity across fragmented demand sources, constructs revenue-aware ad pods, and applies household frequency caps to reduce avoidable repetition.

The two-day MVP deliberately runs as one local Go process. JSON fixtures and a concurrency-safe in-memory store keep it easy to run, test, and reason about without external infrastructure.

## The problem

CTV viewers can receive the same ad multiple times in one break or repeatedly across channels and devices. Ad decisioning must balance viewer experience with incomplete creative identifiers, protected inventory, fixed pod duration, household exposure history, and competing CPMs.

> Given ads from multiple demand sources, inconsistent creative identifiers, household viewing history, contractual restrictions, and competing CPMs, construct an ad pod that minimizes repetition while preserving revenue, fill rate, and low decision latency.

The MVP models these constraints with fixtures and measures local decision latency. Its single-process implementation is not a claim of production-scale latency or multi-instance correctness.

The simulator reports its tradeoffs rather than assuming them. Results are generated from deterministic scenarios, not production results.

## MVP scope

| Capability | Two-day MVP |
|---|---|
| Runtime | Go and the standard library |
| Candidate input | JSON fixtures and simulated VAST 3.0 responses |
| Identity resolution | Synthetic UniversalAdId/ACIF-style metadata, exact-media mappings, and precomputed fingerprint fixtures |
| Frequency state | In-memory `map` protected by `sync.Mutex` |
| Concurrency | Atomic reserve, confirm, and cancel operations within one process |
| Simulation | Seeded, reproducible baseline-versus-optimized runs |
| Verification | Go unit tests, including concurrent reservation tests |

No Docker, Redis, external exchange, video processing, or streaming integration is required to build or run the MVP.

## Architecture

```text
                Seeded Viewing Simulator
                           |
                           v
             JSON Fixtures / VAST 3.0 Candidates
                           |
                           v
                Creative Identity Resolver
                           |
                           v
                  Ad-Pod Decision Engine
        (eligibility, deduplication, CPM selection)
                           |
                           v
            In-Memory FrequencyStore Interface
             map + sync.Mutex; atomic reservations
                           |
                  +--------+--------+
                  |                 |
                  v                 v
        confirm impression     cancel / expire reservation
                  |                 |
                  +--------+--------+
                           v
                Metrics and Comparison Report
```

The frequency store is accessed only through an interface. This keeps the optimizer independent of storage and makes a later shared-state implementation a contained change.

```go
type FrequencyStore interface {
    Reserve(ctx context.Context, householdID, creativeID string,
        limit int, window, ttl time.Duration) (reservationID string, ok bool, err error)
    Confirm(ctx context.Context, reservationID string, at time.Time) error
    Cancel(ctx context.Context, reservationID string) error
}
```

For the MVP, `InMemoryFrequencyStore` holds confirmed impressions and active reservations behind one mutex. `Reserve` prunes expired state, counts confirmed impressions plus active reservations in the rolling window, and records a reservation in the same critical section. This prevents cap oversubscription among goroutines in the single process.

## Decision flow

1. Load candidates from JSON/VAST 3.0 fixtures.
2. Resolve each candidate to a canonical creative where evidence is sufficient.
3. Exclude ineligible candidates and respect protected inventory.
4. Reserve capacity for eligible household/creative pairs.
5. Select the highest-value valid candidates for the pod without avoidable exact-creative duplicates.
6. Cancel reservations for candidates not selected or for incomplete pods.
7. Confirm only impressions marked as shown; cancellation and confirmation are safe to retry.

CPM is a bid rate, not per-impression revenue. Estimated gross revenue for a confirmed impression is `CPM / 1000`.

## Priorities

### P0 — two-day MVP

| Feature | Implementation |
|---|---|
| Synthetic catalog | Deterministic JSON candidates for multiple sources and canonical creatives |
| VAST fixtures | Simulated VAST 3.0 inline responses; no live ad calls |
| Creative resolution | Known ID, exact-media, precomputed fingerprint, or unknown |
| Intra-pod deduplication | Avoid exact-creative duplicates when eligible alternatives exist |
| Household caps | Rolling-window state across simulated channels and devices |
| Atomic reservations | `map` + `sync.Mutex`, reservation TTLs, confirmation, and cancellation |
| Pod selection | Greedy CPM-aware selection subject to duration and inventory constraints |
| Reproducible simulator | Fixed seed, baseline and optimized strategies, shared input scenario |
| Metrics and tests | Repeat, fill, revenue, latency, cap, and identity metrics |

### P1 — after the MVP

- Playback-aware completed-break state for rewind/replay.
- Product-level separation, language eligibility, and ad-safety metadata.
- Decision explainability and a lightweight viewing timeline.
- More complete VAST parsing, wrappers, tracking events, and policy validation.
- Decision budgets and deterministic degraded-mode behavior.

### P2 — scale and production-oriented exploration

- Simulated UID2 and LiveRamp RampID integrations for consent-aware audience identity mapping and cross-device/household frequency-capping experiments.
- Real media processing with FFmpeg, audio fingerprints, and perceptual video hashes.
- VAST 4.x, OpenRTB pod bidding, and registry-backed creative-ID validation.
- Redis-backed frequency storage for shared atomic state across multiple service instances.
- Docker packaging, Kubernetes deployment, load testing, and fault injection.
- Event pipelines and advanced pod optimization.

## Repository layout

```text
adpod-lab/
├── README.md
├── go.mod
├── cmd/simulator/main.go
├── internal/
│   ├── creative/resolver.go
│   ├── frequency/{store.go,memory.go}
│   ├── model/
│   ├── optimizer/
│   ├── simulator/
│   └── metrics/
├── testdata/
│   ├── candidates.json
│   ├── creatives.json
│   └── vast3/
├── scripts/generate_fixtures.go
└── ..._test.go
```

There is no Dockerfile, Compose file, Redis client, or service dependency in the MVP path.

## Run and verify

```bash
go test ./...
go run ./cmd/simulator -seed 20261011
```

Both strategies use the same fixtures and seed.

| Strategy | Identity | Household history | Pod behavior |
|---|---|---|---|
| Baseline | Source-local IDs | None | Highest CPM only |
| Optimized | Canonical IDs when resolved | Shared in-process store | Deduplication, caps, and CPM-aware constraints |

Report at least exact-creative repeat rate, intra-pod duplicate rate, frequency-cap violations, fill rate, confirmed estimated gross revenue, P95 decision latency, and identity resolution precision/recall against fixture ground truth.

## MVP limitations

- The in-memory store is correct only in one Go process and loses data on restart.
- Fixture fingerprints model resolution evidence; they are not extracted from media.
- VAST 3.0 fixtures are controlled input, not a complete parser or live integration.
- A greedy optimizer is intentionally simple and may not find the global optimum.

## Future distributed deployment

The local store is an intentional MVP boundary, not a claim of multi-instance correctness. In a horizontally scaled service, independent process-local maps could each admit exposure capacity for the same household. A future `RedisFrequencyStore` can preserve the interface while using shared atomic reservation operations. Docker becomes useful then for reproducibly running the service and Redis locally; Kubernetes and load tests follow once multiple instances are in scope.

## References

- [IAB Tech Lab VAST](https://iabtechlab.com/standards/vast/)
- [IAB Tech Lab ACIF](https://iabtechlab.com/standards/acif/)
- [Go `sync.Mutex`](https://pkg.go.dev/sync#Mutex)
