# AdPod Lab: Intelligent CTV Ad Pod Optimization
### by Raymond Gan

AdPod Lab is a Go-based CTV ad-decisioning simulation. It finds ad identity across fragmented demand sources, constructs revenue-aware ad pods, and applies household frequency caps to reduce ad repetition.

The two-day MVP deliberately runs as one local Go process. JSON fixtures and a concurrency-safe in-memory store keep it easy to run, test, and reason about without external infrastructure.

## The problem

CTV viewers can get the same ad multiple times in one ad break or repeatedly across channels and devices. Ad decisioning must balance viewer experience with incomplete creative identifiers, protected inventory, fixed pod duration, household exposure history, and competing CPMs.

> Given ads from multiple demand sources, inconsistent creative identifiers, household viewing history, contractual restrictions, and competing CPMs, construct an ad pod that minimizes repetition while preserving revenue, fill rate, and low decision latency.

The MVP models these constraints with fixtures and measures local decision latency. Its single-process implementation doesn't claim production-scale latency or multi-instance correctness.

The simulator reports its tradeoffs rather than assuming them. Results are generated, not real production results.

## MVP scope

| Capability | Two-day MVP |
|---|---|
| Runtime | Go and the standard library |
| Candidate input | JSON candidate, creative, and identity fixtures |
| Identity resolution | Synthetic UniversalAdId/ACIF-style metadata, exact media hash, and fingerprint fixtures |
| Frequency state | In-memory `map` protected by `sync.Mutex` |
| Concurrency | Atomic reserve, confirm, and cancel operations within one process |
| Simulation | Seeded viewing events, playback outcomes, and isolated baseline-versus-optimized replays |
| Metrics | Selection, settlement, revenue, fill, duplicate, and canonical cap metrics |
| Verification | Unit tests plus `go test -race ./...` |

No Docker, Redis, external exchange, video processing, or streaming integration is required to build or run the MVP.

## Architecture

```text
                Seeded Viewing Simulator
                           |
                           v
             JSON Candidate Fixtures
                           |
                           v
                Creative Identity Resolver
                           |
                           v
                Ad Pod Decision Engine
        (protected inventory, CPM selection, deduplication)
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

The simulator generates one immutable event stream, then replays it through
two fresh stores. The baseline uses device-scoped, source-local keys and the
optimized strategy uses household-scoped canonical keys. Both strategies use
the same reservation lifecycle, injected simulated clock, candidate pool, and
playback outcomes.

```go
type FrequencyStore interface {
    Reserve(ctx context.Context, householdID, creativeID string,
        limit int, window, ttl time.Duration) (reservationID string, ok bool, err error)
    Confirm(ctx context.Context, reservationID string, at time.Time) error
    Cancel(ctx context.Context, reservationID string) error
}
```

For the MVP, `InMemoryFrequencyStore` holds confirmed impressions and active reservations behind one mutex. `Reserve` prunes expired state, counts confirmed impressions plus active reservations in the rolling window, and records a reservation in the same critical section. This prevents cap oversubscription among goroutines in the single process.

## Creative identity proof

These fields show different kinds of ad ID proof. They aren't interchangeable. In this project, all data are fake test data.

- `universal_ad_id` is a source-independent creative ID. In this
  project, it's fake test data, not a true [ACIF](https://iabtechlab.com/standards/acif) registry ID. **[UniversalAdId](https://vastlint.org/docs/universal-ad-id/)** is the VAST element that gives an ad a single identity across every system that touches it. Ad servers, DSPs, SSPs, and verification vendors each assign their own internal creative IDs, but none match. The **UniversalAdId** is the one ID that means the same unique ad everywhere.
- `source_creative_id` is assigned by one demand source (Amazon APS,  Google Ad Manager, Publica, FreeWheel...). Different sources can assign different values to the same ad, so it's never enough by itself to prove canonical identity.
- `media_hash` identifies the exact file bytes of a video with a cryptographic hashing algorithm (like `SHA-256` or `MD5`). Test values use `sha256:`-style strings. If 2 video files have equal hashes, they are byte-for-byte identical files. Different hashes may represent different transcodes (720p vs. 1080p) or remuxed files of the same ad.
- `fixture_fingerprint`: Audio/video fingerprinting creates unique mathematical signatures based on how media actually looks and sounds, not relying on file metadata or raw bits. This lets us ID ads even if their files are resized, compressed, or transcoded. Audio fingerprints could use [Chromaprint](https://acoustid.org/chromaprint). Video fingerprints could use [VideoHash](https://github.com/akamhy/videohash).

This resolver should use proof in descending order of trust:
`universal_ad_id` -> exact media hash -> `fixture_fingerprint` -> unknown. A source-local ID, advertiser, or product match does not mean 2 ads are the same.

The fixture resolver loads `data/identity_registry.json`, a separate trusted
mapping from opaque proof values to canonical IDs. It never derives an answer
from a candidate's identifier text, and `data/ground_truth.json` is evaluation
data only. This prevents the synthetic test data from leaking canonical IDs
through strings such as `fingerprint-creative-003`.

### Scenarios the resolver must distinguish

The current test fixtures keep the three identity signals separate. Check
`universal_ad_id` for trusted identifier proof, `media_hash` for exact-file
proof, and `fixture_fingerprint` for the simulated audiovisual-content
signal.

| Scenario | `universal_ad_id` | `media_hash` | `fixture_fingerprint` | Same underlying ad? | Expected result |
|---|---|---|---|---|---|
| Exact same file | Same or trusted | Same | Same | Yes | Resolve to 1 canonical creative |
| Same file, different source IDs | Missing or different | Same | Same | Yes | Match on exact media hash |
| Same commercial, different transcodes | Missing | Different | Same | Yes | Match on fixture fingerprint |
| Different commercials, same product | Missing or different | Different | Different | No | Keep canonical creatives distinct |
| Different commercials, same advertiser | Missing or different | Different | Different | No | Same advertiser does NOT mean same ad |
| Unknown creative identity | Missing | Different | Missing | Unknown | Return unresolved/unknown |

For example, two Toyota RAV4 candidates can share `product_id: "rav4"` while
mapping to different canonical creative IDs. They must not be deduplicated
just because they advertise the same product. Conversely, two source-local
Toyota IDs with different transcodes could still be the same ad if their trusted universal ad IDs or fixture fingerprints match.

The 30-candidate fixture has 27 candidates with sufficient proof and three
explicitly expected-unknown candidates. Its Toyota scenario is **four
candidates spanning two distinct RAV4 commercials**: candidates 024/025 share
one fingerprint and map to `creative-003`; 026/027 share another and map to
`creative-004`. The two groups must remain distinct despite identical
advertiser and product metadata.

## Decision flow

1. Load the fixed 30-candidate catalog and trusted identity registry.
2. Generate a deterministic viewing timeline from the supplied seed.
3. Resolve candidates without consulting evaluation-only ground truth.
4. Replay the same timeline independently for the baseline and optimized strategies.
5. Reserve capacity, select a protected 120-second pod, and settle each slot.
6. Apply cancellations and confirmations at their simulated timestamps.
7. Evaluate canonical duplicates and cap violations afterward using ground truth.

CPM is a bid rate, not per-impression revenue. Estimated gross revenue for a confirmed impression is `CPM / 1000`.

## Priorities

### P0 — two-day MVP

| Feature | Implementation |
|---|---|
| Synthetic catalog | Deterministic JSON candidates for multiple sources and canonical creatives |
| VAST fixtures | Simulated VAST 3.0 inline responses; no live ad calls |
| Creative resolution | Known Universal Ad ID, exact media hash, precomputed fingerprint, or unknown |
| Intra-pod deduplication | Avoid exact-creative duplicates when eligible alternatives exist |
| Household caps | Rolling-window state across simulated channels and devices |
| Atomic reservations | `map` + `sync.Mutex`, reservation TTLs, confirmation, and cancellation |
| Pod selection | Greedy CPM-aware selection subject to duration and inventory constraints |
| Reproducible simulator | Fixed seed, immutable event stream, isolated strategy replays |
| Playback lifecycle | Confirmed, canceled, delayed, and expired reservations |
| Metrics and tests | Settlement, fill, revenue, duplicate, cap, identity, and determinism metrics |

### P1 — after the MVP

- Playback-aware completed-break state for rewind/replay.
- Product-level separation, language eligibility, and ad-safety metadata.
- Decision explainability and a lightweight viewing timeline.
- More complete VAST parsing, wrappers, tracking events, and policy validation.
- Decision budgets and deterministic degraded-mode behavior.

### P2 — scale and production-oriented exploration

- Simulated [UID2](https://www.publift.com/blog/unified-id-2-0#what-is-unified-id-20) and [LiveRamp RampID](https://liveramp.com/our-platform/rampid) integrations for consent-aware audience identity mapping and cross-device/household frequency-capping experiments.
- Real media processing with FFmpeg, audio fingerprints, and perceptual video hashes.
- VAST 4.x, OpenRTB pod bidding, and registry-backed creative-ID validation.
- Redis-backed frequency storage for shared atomic state across multiple service instances.
- Docker packaging, Kubernetes deployment, load testing, and fault injection.
- Event pipelines and advanced pod optimization.

## Repository layout

```text
adpod-lab/
├── README.md
├── data/{candidates,creatives,identity_registry,ground_truth}.json
├── go.mod
├── cmd/simulator/main.go
├── internal/
│   ├── creative/resolver.go
│   ├── frequency/{store.go,memory.go}
│   ├── model/
│   ├── pod/selector.go
│   └── simulator/{simulator.go,simulator_test.go}
├── cmd/generate-fixtures/main.go
└── ..._test.go
```

There is no Dockerfile, Compose file, Redis client, or service dependency in the MVP path.

## Run and verify

```bash
go test ./...
go test -race ./...
go run ./cmd/simulator \
  -seed 20261011 \
  -households 20 \
  -devices 3 \
  -channels 5 \
  -minutes 120
```

The default run uses 20 households, up to three devices per household, five
channels, and two simulated hours. Use `-households 100` to run the larger
scenario. The seed controls viewing behavior and playback outcomes; the
committed candidate fixtures remain the shared inventory for both strategies.

### Sample simulator output:

```text
AdPod Lab Simulator
  Seed: 20261011  Households: 20  Devices: <=3  Channels: 5  Minutes: 120
  Candidates: 30  Events: 1944

Metric                                       baseline      optimized
Processed breaks                                  361            361
Ad opportunities                                 1444           1444
Selected ads                                     1444           1332
Confirmed impressions                            1222           1117
Canceled impressions                              151            146
Delayed confirmations                              62             58
Expired/unknown settlements                        71             69
Incomplete pods                                     0             28
Intra-pod duplicates                              226              0
Frequency-cap violations                          613              0
Estimated revenue                            $48.8221       $41.4609
Confirmed revenue                            $41.1716       $34.6367
Fill rate                                     100.00%         92.24%
Unresolved identities                               3              3
```

### Output terms:

- `Seed`: fixed value that makes generated simulation choices reproducible.
- `Events`: immutable viewing, break, channel-switch, and playback events
  generated before either strategy runs.
- `Ad opportunities`: four available slots per processed 120-second break.
- `Selected ads`: candidates selected before settlement; their CPM contributes
  to estimated revenue even if playback is later canceled.
- `Confirmed revenue`: CPM/1,000 for successfully confirmed impressions only.
- `Delayed confirmations`: delayed playback outcomes that successfully confirm.
  Expired delayed reservations are reported separately.
- `Intra-pod duplicates`: repeated canonical creatives in one pod, evaluated
  with ground-truth IDs after replay.
- `Frequency-cap violations`: each confirmed impression beyond the configured
  household/canonical-creative rolling-window cap, counted once.
- `Fill rate`: selected slots divided by available slots.
- `Incomplete pods`: breaks that could not fill all four slots after applying
  frequency and identity constraints.

Both strategies use the same reserve/confirm/cancel lifecycle. The baseline
uses device-scoped source-local keys and does not canonical-dedupe within a
pod. The optimized path uses household-scoped canonical keys, canonical
intra-pod deduplication, and rolling-window caps. This isolates identity scope
and household sharing instead of making the baseline intentionally unsafe.

The simulator reports actual tradeoffs; it does not hard-code an expected
revenue or fill improvement.

## MVP limitations

- The in-memory store is correct only in one Go process and loses data on restart.
- Fixture fingerprints model resolution proof; they are not extracted from media.
- JSON fixtures are controlled input; VAST parsing and live integrations are
  intentionally out of scope.
- A greedy optimizer is intentionally simple and may not find the global optimum.
- Decision-latency percentiles are not part of the current compact report;
  the simulation prioritizes deterministic lifecycle and outcome metrics.

## Future distributed deployment

The local store is an intentional MVP boundary, not a claim of multi-instance correctness. In a horizontally scaled service, independent process-local maps could each admit exposure capacity for the same household. A future `RedisFrequencyStore` can preserve the interface while using shared atomic reservation operations. Docker becomes useful then for reproducibly running the service and Redis locally; Kubernetes and load tests follow once multiple instances are in scope.

## References

- [IAB Tech Lab VAST](https://iabtechlab.com/standards/vast/)
- [IAB Tech Lab ACIF](https://iabtechlab.com/standards/acif/)
- [Go `sync.Mutex`](https://pkg.go.dev/sync#Mutex)
