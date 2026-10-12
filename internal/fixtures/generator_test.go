package fixtures

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rgan/adpod-lab/internal/model"
)

func TestGenerateIsDeterministic(t *testing.T) {
	first := generateJSON(t, Seed)
	second := generateJSON(t, Seed)
	if !bytes.Equal(first, second) {
		t.Fatal("generating fixtures twice with the same seed produced different JSON")
	}
}

func TestGeneratedFixtureFiles(t *testing.T) {
	root := filepath.Join("..", "..", "data")
	creatives, err := model.LoadCreativesFile(filepath.Join(root, "creatives.json"))
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := model.LoadCandidatesFile(filepath.Join(root, "candidates.json"))
	if err != nil {
		t.Fatal(err)
	}
	groundTruth, err := model.LoadGroundTruthFile(filepath.Join(root, "ground_truth.json"))
	if err != nil {
		t.Fatal(err)
	}

	if len(creatives) != 12 || len(candidates) != 30 || len(groundTruth) != 30 {
		t.Fatalf("fixture sizes = creatives %d, candidates %d, ground truth %d; want 12, 30, 30",
			len(creatives), len(candidates), len(groundTruth))
	}
	creativeIDs := make(map[string]bool, len(creatives))
	for _, creative := range creatives {
		if creative.CreativeID == "" || creative.AdvertiserName == "" || creative.ProductName == "" ||
			creative.DurationSeconds != 30 {
			t.Fatalf("invalid creative schema: %+v", creative)
		}
		if creativeIDs[creative.CreativeID] {
			t.Fatalf("duplicate creative ID %q", creative.CreativeID)
		}
		creativeIDs[creative.CreativeID] = true
	}

	scenarioCounts := make(map[string]int)
	candidateIDs := make(map[string]bool, len(candidates))
	sourceCreativeIDs := make(map[string]map[string]bool)
	sourceMediaURLs := make(map[string]map[string]bool)
	for _, candidate := range candidates {
		if candidate.CandidateID == "" || candidate.Source == "" || candidate.SourceCreativeID == "" ||
			candidate.MediaURL == "" || candidate.AdvertiserID == "" || candidate.ProductID == "" ||
			candidate.DurationSeconds != 30 || candidate.CPM <= 0 {
			t.Fatalf("invalid candidate schema: %+v", candidate)
		}
		if candidateIDs[candidate.CandidateID] {
			t.Fatalf("duplicate candidate ID %q", candidate.CandidateID)
		}
		candidateIDs[candidate.CandidateID] = true
		if sourceCreativeIDs[candidate.Source] == nil {
			sourceCreativeIDs[candidate.Source] = map[string]bool{}
			sourceMediaURLs[candidate.Source] = map[string]bool{}
		}
		if sourceCreativeIDs[candidate.Source][candidate.SourceCreativeID] {
			t.Fatalf("source creative ID %q is not unique within %s", candidate.SourceCreativeID, candidate.Source)
		}
		if sourceMediaURLs[candidate.Source][candidate.MediaURL] {
			t.Fatalf("media URL %q is not unique within %s", candidate.MediaURL, candidate.Source)
		}
		sourceCreativeIDs[candidate.Source][candidate.SourceCreativeID] = true
		sourceMediaURLs[candidate.Source][candidate.MediaURL] = true
		scenarioCounts[candidate.Scenario]++

		expectedCreative, ok := groundTruth[candidate.CandidateID]
		if !ok || !creativeIDs[expectedCreative] {
			t.Fatalf("ground truth missing or invalid for %s", candidate.CandidateID)
		}
	}

	wantCounts := map[string]int{
		ScenarioUniversal:    9,
		ScenarioExactMedia:   8,
		ScenarioFingerprint:  6,
		ScenarioSameProduct:  4,
		ScenarioInsufficient: 3,
	}
	for scenario, want := range wantCounts {
		if scenarioCounts[scenario] != want {
			t.Errorf("scenario %q count = %d, want %d", scenario, scenarioCounts[scenario], want)
		}
	}
}

func TestEvidenceScenarios(t *testing.T) {
	_, candidates, groundTruth := Generate(Seed)
	sameProductCreatives := map[string]bool{}
	sameProductProducts := map[string]bool{}
	for _, candidate := range candidates {
		switch candidate.Scenario {
		case ScenarioUniversal:
			if candidate.UniversalAdID == "" {
				t.Errorf("%s has no universal ID", candidate.CandidateID)
			}
		case ScenarioExactMedia:
			if candidate.UniversalAdID != "" || candidate.MediaKey == "" || candidate.FixtureFingerprint != "" {
				t.Errorf("%s does not have exact-media-only evidence", candidate.CandidateID)
			}
		case ScenarioFingerprint:
			if candidate.UniversalAdID != "" || candidate.MediaKey == "" || candidate.FixtureFingerprint == "" {
				t.Errorf("%s does not have fingerprint evidence", candidate.CandidateID)
			}
		case ScenarioSameProduct:
			if candidate.UniversalAdID != "" {
				t.Errorf("%s unexpectedly has a universal ID", candidate.CandidateID)
			}
			sameProductCreatives[groundTruth[candidate.CandidateID]] = true
			sameProductProducts[candidate.ProductID] = true
		case ScenarioInsufficient:
			if candidate.UniversalAdID != "" || candidate.MediaKey != "" || candidate.FixtureFingerprint != "" {
				t.Errorf("%s has identity evidence", candidate.CandidateID)
			}
		default:
			t.Errorf("%s has unknown scenario %q", candidate.CandidateID, candidate.Scenario)
		}
	}
	if len(sameProductCreatives) != 2 || len(sameProductProducts) != 1 {
		t.Errorf("same-product scenario has %d creatives across %d products, want 2 creatives across 1 product",
			len(sameProductCreatives), len(sameProductProducts))
	}
}

func generateJSON(t *testing.T, seed int64) []byte {
	t.Helper()
	creatives, candidates, groundTruth := Generate(seed)
	var buffer bytes.Buffer
	if err := WriteJSON(&buffer, struct {
		Creatives   []model.Creative  `json:"creatives"`
		Candidates  []model.Candidate `json:"candidates"`
		GroundTruth map[string]string `json:"ground_truth"`
	}{creatives, candidates, groundTruth}); err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(buffer.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestGeneratedFilesAreCommitted(t *testing.T) {
	for _, name := range []string{"creatives.json", "candidates.json", "ground_truth.json"} {
		if _, err := os.Stat(filepath.Join("..", "..", "data", name)); err != nil {
			t.Fatalf("missing generated fixture %s: %v", name, err)
		}
	}
}
