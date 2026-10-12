package creative

import (
	"path/filepath"
	"testing"

	"github.com/rgan/adpod-lab/internal/model"
)

func TestResolveProofPrecedence(t *testing.T) {
	candidate := model.Candidate{
		CandidateID:        "candidate-1",
		UniversalAdID:      "u-1",
		MediaHash:          "m-1",
		FixtureFingerprint: "f-1",
	}
	resolver := NewResolver(IdentityRegistry{
		UniversalIDs: map[string]string{"u-1": "creative-1"},
		MediaHashes:  map[string]string{"m-1": "creative-1"},
		Fingerprints: map[string]string{"f-1": "creative-1"},
	})

	got := resolver.Resolve(candidate)
	if got.CanonicalCreativeID != "creative-1" || got.Method != string(MethodUniversalID) || got.Confidence != 1 {
		t.Fatalf("Resolve() = %+v, want universal-ID resolution", got)
	}
}

func TestFixtureRegistryResolvesKnownCandidatesAndAbstainsFromExpectedUnknowns(t *testing.T) {
	root := filepath.Join("..", "..", "data")
	candidates, err := model.LoadCandidatesFile(filepath.Join(root, "candidates.json"))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := model.LoadIdentityRegistryFile(filepath.Join(root, "identity_registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	expectations, err := model.LoadGroundTruthFile(filepath.Join(root, "ground_truth.json"))
	if err != nil {
		t.Fatal(err)
	}
	metrics := Evaluate(NewResolver(registry).ResolveAll(candidates), expectations)
	if metrics.TruePositives != 27 || metrics.FalsePositives != 0 || metrics.FalseNegatives != 0 ||
		metrics.CorrectUnknowns != 3 || metrics.ExpectedUnknowns != 3 {
		t.Fatalf("fixture metrics = %+v, want 27 correct known resolutions and 3 correct unknowns", metrics)
	}
}

func TestResolveScenarios(t *testing.T) {
	tests := []struct {
		name       string
		candidate  model.Candidate
		wantID     string
		wantMethod ResolutionMethod
	}{
		{
			name: "different source IDs same exact file",
			candidate: model.Candidate{
				CandidateID:      "exact",
				SourceCreativeID: "source-b-99",
				MediaHash:        "media-1",
			},
			wantID: "creative-1", wantMethod: MethodExactMedia,
		},
		{
			name: "transcode fingerprint",
			candidate: model.Candidate{
				CandidateID:        "transcode",
				MediaHash:          "different-bytes",
				FixtureFingerprint: "fingerprint-1",
			},
			wantID: "creative-1", wantMethod: MethodFingerprint,
		},
		{
			name: "same product stays distinct",
			candidate: model.Candidate{
				CandidateID:  "distinct",
				AdvertiserID: "toyota",
				ProductID:    "rav4",
				MediaHash:    "media-2",
			},
			wantID: "creative-2", wantMethod: MethodExactMedia,
		},
		{
			name: "unknown identity",
			candidate: model.Candidate{
				CandidateID:      "unknown",
				AdvertiserID:     "toyota",
				ProductID:        "rav4",
				SourceCreativeID: "source-a-1",
			},
			wantMethod: MethodUnknown,
		},
	}
	resolver := NewResolver(IdentityRegistry{
		MediaHashes:  map[string]string{"media-1": "creative-1", "media-2": "creative-2"},
		Fingerprints: map[string]string{"fingerprint-1": "creative-1", "fingerprint-2": "creative-3"},
	})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := resolver.Resolve(test.candidate)
			if got.CanonicalCreativeID != test.wantID || got.Method != string(test.wantMethod) {
				t.Fatalf("Resolve() = %+v, want ID %q and method %q", got, test.wantID, test.wantMethod)
			}
		})
	}
}

func TestResolveConflictingProofIsUnknown(t *testing.T) {
	resolver := NewResolver(IdentityRegistry{
		UniversalIDs: map[string]string{"u-1": "creative-1"},
		MediaHashes:  map[string]string{"m-1": "creative-2"},
	})
	got := resolver.Resolve(model.Candidate{
		CandidateID: "conflict", UniversalAdID: "u-1", MediaHash: "m-1",
	})
	if got.CanonicalCreativeID != "" || got.Method != string(MethodConflict) || got.Confidence != 0 {
		t.Fatalf("Resolve() = %+v, want unresolved conflict", got)
	}
}

func TestEvaluateSeparatesExpectedUnknowns(t *testing.T) {
	candidates := []model.Candidate{
		{CandidateID: "u", UniversalAdID: "u-opaque"},
		{CandidateID: "m", MediaHash: "m-opaque"},
		{CandidateID: "f", MediaHash: "different-bytes", FixtureFingerprint: "f-opaque"},
		{CandidateID: "x", MediaHash: "sha256:unknown"},
	}
	predictions := NewResolver(IdentityRegistry{
		UniversalIDs: map[string]string{"u-opaque": "creative-001"},
		MediaHashes:  map[string]string{"m-opaque": "creative-002"},
		Fingerprints: map[string]string{"f-opaque": "creative-001"},
	}).ResolveAll(candidates)
	metrics := Evaluate(predictions, model.ResolutionExpectations{
		Resolved:            map[string]string{"u": "creative-001", "m": "creative-002", "f": "creative-001"},
		UnknownCandidateIDs: []string{"x"},
	})
	if metrics.TruePositives != 3 || metrics.FalseNegatives != 0 || metrics.CorrectUnknowns != 1 ||
		metrics.ExpectedUnknowns != 1 || metrics.Precision != 1 || metrics.Recall != 1 {
		t.Fatalf("metrics = %+v, want three known resolutions and one correct unknown", metrics)
	}
}
