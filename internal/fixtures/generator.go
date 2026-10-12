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
	ScenarioInsufficient = "insufficient_identity_evidence"
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

// Generate returns the canonical creatives, candidates, and test-only ground
// truth. The seed controls only generated bid and inventory metadata; the
// identity relationships are explicit and stable.
func Generate(seed int64) ([]model.Creative, []model.Candidate, map[string]string) {
	rng := rand.New(rand.NewSource(seed))
	candidates := make([]model.Candidate, 0, len(candidateSpecs))
	groundTruth := make(map[string]string, len(candidateSpecs))
	sources := []string{"source-a", "source-b", "source-c"}

	for i, spec := range candidateSpecs {
		creative := catalog[spec.creative]
		source := sources[i%len(sources)]
		candidateID := fmt.Sprintf("candidate-%03d", i+1)
		mediaKey := ""
		fingerprint := ""
		universalID := ""
		switch spec.scenario {
		case ScenarioUniversal:
			universalID = "synthetic-universal-" + creative.CreativeID
		case ScenarioExactMedia:
			mediaKey = "sha256:exact-" + creative.CreativeID
		case ScenarioFingerprint:
			mediaKey = fmt.Sprintf("sha256:transcode-%s-%03d", creative.CreativeID, i+1)
			fingerprint = "fixture-fingerprint-" + creative.CreativeID
		case ScenarioSameProduct:
			mediaKey = fmt.Sprintf("sha256:distinct-creative-%03d", i+1)
		case ScenarioInsufficient:
			// No universal ID, exact hash, or fingerprint is intentional.
		}

		candidates = append(candidates, model.Candidate{
			CandidateID:        candidateID,
			Source:             source,
			SourceCreativeID:   fmt.Sprintf("%s-creative-%03d", source, i+1),
			UniversalAdID:      universalID,
			MediaURL:           fmt.Sprintf("https://media.invalid/%s/%s.mp4", source, candidateID),
			MediaKey:           mediaKey,
			FixtureFingerprint: fingerprint,
			AdvertiserID:       creative.AdvertiserID,
			ProductID:          creative.ProductID,
			DurationSeconds:    creative.DurationSeconds,
			CPM:                float64(1800+rng.Intn(2200)) / 100,
			InventoryOwner:     source + "-inventory",
			Replaceable:        rng.Intn(5) != 0,
			Scenario:           spec.scenario,
		})
		groundTruth[candidateID] = creative.CreativeID
	}
	return append([]model.Creative(nil), catalog...), candidates, groundTruth
}

// WriteJSON writes one deterministic fixture as indented JSON.
func WriteJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
