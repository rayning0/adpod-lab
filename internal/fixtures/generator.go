// Package fixtures generates the deterministic metadata used by the MVP.
package fixtures

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"

	"github.com/rgan/adpod-lab/internal/model"
)

const (
	Seed = int64(20261011)

	ScenarioUniversal    = "valid_universal_id"
	ScenarioExactMedia   = "exact_media_hash"
	ScenarioFingerprint  = "fixture_fingerprint"
	ScenarioSameProduct  = "same_product_different_creative"
	ScenarioInsufficient = "insufficient_identity_proof"
)

type candidateSpec struct {
	scenario string
	creative int
}

var catalog = []model.Creative{
	{CreativeID: "creative-001", Title: "Big Mac - The Original", AdvertiserID: "mcdonalds", AdvertiserName: "McDonald's", ProductID: "big-mac", ProductName: "Big Mac", DurationSeconds: 30},
	{CreativeID: "creative-002", Title: "Big Mac - Made for Sharing", AdvertiserID: "mcdonalds", AdvertiserName: "McDonald's", ProductID: "big-mac", ProductName: "Big Mac", DurationSeconds: 30},
	{CreativeID: "creative-003", Title: "RAV4 - Go Places", AdvertiserID: "toyota", AdvertiserName: "Toyota", ProductID: "rav4", ProductName: "RAV4", DurationSeconds: 30},
	{CreativeID: "creative-004", Title: "RAV4 - Ready for More", AdvertiserID: "toyota", AdvertiserName: "Toyota", ProductID: "rav4", ProductName: "RAV4", DurationSeconds: 30},
	{CreativeID: "creative-005", Title: "iPhone - Shot on iPhone", AdvertiserID: "apple", AdvertiserName: "Apple", ProductID: "iphone", ProductName: "iPhone", DurationSeconds: 30},
	{CreativeID: "creative-006", Title: "iPhone - Your Next Chapter", AdvertiserID: "apple", AdvertiserName: "Apple", ProductID: "iphone", ProductName: "iPhone", DurationSeconds: 30},
	{CreativeID: "creative-007", Title: "Air Max - Move to Zero", AdvertiserID: "nike", AdvertiserName: "Nike", ProductID: "air-max", ProductName: "Air Max", DurationSeconds: 30},
	{CreativeID: "creative-008", Title: "Air Max - Find Your Fast", AdvertiserID: "nike", AdvertiserName: "Nike", ProductID: "air-max", ProductName: "Air Max", DurationSeconds: 30},
	{CreativeID: "creative-009", Title: "Coca-Cola - Open Happiness", AdvertiserID: "coca-cola", AdvertiserName: "Coca-Cola", ProductID: "coca-cola", ProductName: "Coca-Cola", DurationSeconds: 30},
	{CreativeID: "creative-010", Title: "Coca-Cola - Share a Coke", AdvertiserID: "coca-cola", AdvertiserName: "Coca-Cola", ProductID: "coca-cola", ProductName: "Coca-Cola", DurationSeconds: 30},
	{CreativeID: "creative-011", Title: "Disney+ - Stories for Everyone", AdvertiserID: "disney", AdvertiserName: "Disney+", ProductID: "disney-plus", ProductName: "Disney+", DurationSeconds: 30},
	{CreativeID: "creative-012", Title: "Disney+ - Every Story Starts Here", AdvertiserID: "disney", AdvertiserName: "Disney+", ProductID: "disney-plus", ProductName: "Disney+", DurationSeconds: 30},
}

var candidateSpecs = []candidateSpec{
	{ScenarioUniversal, 0}, {ScenarioUniversal, 2}, {ScenarioUniversal, 4},
	{ScenarioUniversal, 6}, {ScenarioUniversal, 8}, {ScenarioUniversal, 10},
	{ScenarioUniversal, 1}, {ScenarioUniversal, 3}, {ScenarioUniversal, 5},
	{ScenarioExactMedia, 0}, {ScenarioExactMedia, 2}, {ScenarioExactMedia, 4},
	{ScenarioExactMedia, 6}, {ScenarioExactMedia, 8}, {ScenarioExactMedia, 10},
	{ScenarioExactMedia, 1}, {ScenarioExactMedia, 3},
	{ScenarioFingerprint, 0}, {ScenarioFingerprint, 2}, {ScenarioFingerprint, 4},
	{ScenarioFingerprint, 6}, {ScenarioFingerprint, 8}, {ScenarioFingerprint, 10},
	{ScenarioSameProduct, 2}, {ScenarioSameProduct, 2}, {ScenarioSameProduct, 3}, {ScenarioSameProduct, 3},
	{ScenarioInsufficient, 9}, {ScenarioInsufficient, 11}, {ScenarioInsufficient, 1},
}

// These values intentionally do not reveal a canonical creative ID. Their
// meaning comes only from the separate trusted identity registry.
var universalSignals = []string{
	"urn:uuid:4c47f4a2-91c2-4e94-8af1-2e371ec8d001", "urn:uuid:6a5b7c31-2d8e-4f06-9b2d-4a0f17bc9002",
	"urn:uuid:8e90d5e7-63b1-4ae8-94c6-5e4b2df3a003", "urn:uuid:b2d8a601-f3be-41ea-8c17-6df207a4b004",
	"urn:uuid:c1f4e7a2-58d0-4f65-a3b9-7c20e54d5005", "urn:uuid:d5a72c39-84e1-4bc3-90f6-1e8a4bd76006",
	"urn:uuid:e7b0d681-3fc5-4c8a-b9e2-9d541a6f7007", "urn:uuid:f39c1a54-6e2d-44b7-80d5-2a7ce18b8008",
	"urn:uuid:19d6e2c8-7ab4-4f03-a1ce-3b85d97f9009", "urn:uuid:2f8a5c73-1d94-4b60-92e7-4c16af0d1010",
	"urn:uuid:3b7e9d20-5f61-4a8c-b403-6d29e5c21111", "urn:uuid:47c1f8a6-9d30-4e75-8b12-0a3d6f4e1212",
}

var exactMediaSignals = []string{
	"sha256:ad9e7c41b8f02d65", "sha256:1f4a8e92c6b73d50", "sha256:7c20d5a19e4b8f63", "sha256:3b6f1e84a2c95d70",
	"sha256:92e4c7b15d80a6f3", "sha256:5d1a9f38c4e72b60", "sha256:e6b3d0a71f95c842", "sha256:0c8e4a26b7d19f53",
}

var fingerprintSignals = []string{
	"fp:84c1e9a7d2", "fp:31b6f0c8a4", "fp:d7a3c5e1b9", "fp:5e8d2a7c40",
	"fp:a9c4f16e73", "fp:2d7b8e50c1", "fp:6f1a9d4c82", "fp:b3e7c0a519",
}

// Generate returns the canonical creatives, candidates, and test-only ground
// truth. The seed controls only generated bid and inventory metadata; the
// identity relationships are explicit and stable.
func Generate(seed int64) ([]model.Creative, []model.Candidate, model.IdentityRegistry, model.ResolutionExpectations) {
	rng := rand.New(rand.NewSource(seed))
	candidates := make([]model.Candidate, 0, len(candidateSpecs))
	registry := model.IdentityRegistry{UniversalIDs: map[string]string{}, MediaHashes: map[string]string{}, Fingerprints: map[string]string{}}
	expectations := model.ResolutionExpectations{Resolved: map[string]string{}}
	sources := []string{"source-a", "source-b", "source-c"}

	for i, spec := range candidateSpecs {
		creative := catalog[spec.creative]
		source := sources[i%len(sources)]
		candidateID := fmt.Sprintf("candidate-%03d", i+1)
		mediaHash := ""
		fingerprint := ""
		universalID := ""
		switch spec.scenario {
		case ScenarioUniversal:
			universalID = universalSignals[i]
			registry.UniversalIDs[universalID] = creative.CreativeID
		case ScenarioExactMedia:
			mediaHash = exactMediaSignals[i-9]
			registry.MediaHashes[mediaHash] = creative.CreativeID
		case ScenarioFingerprint:
			mediaHash = fmt.Sprintf("sha256:transcode-%03d", i+1)
			fingerprint = fingerprintSignals[i-17]
			registry.Fingerprints[fingerprint] = creative.CreativeID
		case ScenarioSameProduct:
			mediaHash = fmt.Sprintf("sha256:transcode-%03d", i+1)
			// 024/025 are one Toyota commercial; 026/027 are another. Each
			// pair has different file bytes but one shared audiovisual proof.
			if spec.creative == 2 {
				fingerprint = fingerprintSignals[6]
			} else {
				fingerprint = fingerprintSignals[7]
			}
			registry.Fingerprints[fingerprint] = creative.CreativeID
		case ScenarioInsufficient:
			// No universal ID, exact media hash, or fingerprint is intentional.
		}

		candidates = append(candidates, model.Candidate{
			CandidateID:        candidateID,
			Source:             source,
			SourceCreativeID:   fmt.Sprintf("%s-creative-%03d", source, i+1),
			UniversalAdID:      universalID,
			MediaURL:           fmt.Sprintf("https://media.invalid/%s/%s.mp4", source, candidateID),
			MediaHash:          mediaHash,
			FixtureFingerprint: fingerprint,
			AdvertiserID:       creative.AdvertiserID,
			ProductID:          creative.ProductID,
			DurationSeconds:    creative.DurationSeconds,
			CPM:                float64(1800+rng.Intn(2200)) / 100,
			InventoryOwner:     source + "-inventory",
			Replaceable:        rng.Intn(5) != 0,
			Scenario:           spec.scenario,
		})
		if spec.scenario == ScenarioInsufficient {
			expectations.UnknownCandidateIDs = append(expectations.UnknownCandidateIDs, candidateID)
		} else {
			expectations.Resolved[candidateID] = creative.CreativeID
		}
	}
	return append([]model.Creative(nil), catalog...), candidates, registry, expectations
}

// WriteJSON writes one deterministic fixture as indented JSON.
func WriteJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
