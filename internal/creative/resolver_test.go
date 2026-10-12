package creative

import (
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

func TestFixtureResolverMatchesGroundTruth(t *testing.T) {
	creatives := []model.Creative{
		{CreativeID: "creative-001"},
		{CreativeID: "creative-002"},
	}
	candidates := []model.Candidate{
		{CandidateID: "u", UniversalAdID: "synthetic-universal-creative-001"},
		{CandidateID: "m", MediaHash: "sha256:exact-creative-002"},
		{CandidateID: "f", MediaHash: "sha256:transcode-creative-001-3", FixtureFingerprint: "fixture-fingerprint-creative-001"},
		{CandidateID: "x", MediaHash: "sha256:unknown"},
	}
	predictions := NewFixtureResolver(candidates, creatives).ResolveAll(candidates)
	metrics := Evaluate(predictions, map[string]string{
		"u": "creative-001", "m": "creative-002", "f": "creative-001", "x": "creative-002",
	})
	if metrics.TruePositives != 3 || metrics.FalseNegatives != 1 || metrics.Precision != 1 || metrics.Recall != 0.75 {
		t.Fatalf("metrics = %+v, want 3 TP, 1 FN, precision 1, recall .75", metrics)
	}
}
