// Package creative resolves source candidates to canonical commercial IDs.
package creative

import "github.com/rgan/adpod-lab/internal/model"

// ResolutionMethod describes the proof used to resolve a candidate.
type ResolutionMethod string

const (
	MethodUniversalID ResolutionMethod = "universal_ad_id"
	MethodExactMedia  ResolutionMethod = "exact_media_hash"
	MethodFingerprint ResolutionMethod = "fixture_fingerprint"
	MethodUnknown     ResolutionMethod = "unknown"
	MethodConflict    ResolutionMethod = "conflicting_proof"
)

// IdentityRegistry contains trusted mappings from identity signals to
// canonical creative IDs. In production these mappings would come from a
// validated creative registry; the MVP uses synthetic fixture mappings.
type IdentityRegistry = model.IdentityRegistry

// Resolver applies identity proof in descending order of trust.
type Resolver struct {
	registry IdentityRegistry
}

// NewResolver creates a resolver using the supplied trusted identity maps.
func NewResolver(registry IdentityRegistry) *Resolver {
	return &Resolver{registry: registry}
}

// Resolve returns the best-supported canonical identity for candidate.
// Conflicting recognized proof is intentionally unresolved rather than
// silently allowing a lower-trust signal to override another signal.
func (r *Resolver) Resolve(candidate model.Candidate) model.ResolvedCreative {
	type proof struct {
		method     ResolutionMethod
		canonical  string
		confidence float64
	}
	proofs := make([]proof, 0, 3)
	if id := r.registry.UniversalIDs[candidate.UniversalAdID]; candidate.UniversalAdID != "" && id != "" {
		proofs = append(proofs, proof{MethodUniversalID, id, 1.0})
	}
	if id := r.registry.MediaHashes[candidate.MediaHash]; candidate.MediaHash != "" && id != "" {
		proofs = append(proofs, proof{MethodExactMedia, id, 0.99})
	}
	if id := r.registry.Fingerprints[candidate.FixtureFingerprint]; candidate.FixtureFingerprint != "" && id != "" {
		proofs = append(proofs, proof{MethodFingerprint, id, 0.96})
	}

	if len(proofs) == 0 {
		return model.ResolvedCreative{
			CandidateID: candidate.CandidateID,
			Method:      string(MethodUnknown),
		}
	}
	for _, p := range proofs[1:] {
		if p.canonical != proofs[0].canonical {
			return model.ResolvedCreative{
				CandidateID: candidate.CandidateID,
				Method:      string(MethodConflict),
			}
		}
	}

	return model.ResolvedCreative{
		CandidateID:         candidate.CandidateID,
		CanonicalCreativeID: proofs[0].canonical,
		Method:              string(proofs[0].method),
		Confidence:          proofs[0].confidence,
	}
}

// ResolveAll resolves candidates in input order.
func (r *Resolver) ResolveAll(candidates []model.Candidate) []model.ResolvedCreative {
	resolved := make([]model.ResolvedCreative, 0, len(candidates))
	for _, candidate := range candidates {
		resolved = append(resolved, r.Resolve(candidate))
	}
	return resolved
}

// ResolutionMetrics reports candidate-level identity outcomes against
// canonical IDs. The categories are mutually exclusive for each candidate.
type ResolutionMetrics struct {
	CorrectResolvedKnownIdentities int
	CorrectResolvedUnknowns        int
	MissedKnownIdentities          int
	WrongResolvedUnknowns          int
	WrongIdentityAssignments       int
	ExpectedUnknowns               int
}

// Evaluate compares predictions with test-only ground truth.
func Evaluate(predictions []model.ResolvedCreative, groundTruth model.ResolutionExpectations) ResolutionMetrics {
	var metrics ResolutionMetrics
	expectedUnknown := make(map[string]bool, len(groundTruth.UnknownCandidateIDs))
	for _, candidateID := range groundTruth.UnknownCandidateIDs {
		expectedUnknown[candidateID] = true
	}
	for _, prediction := range predictions {
		expected, known := groundTruth.Resolved[prediction.CandidateID]
		if !known && !expectedUnknown[prediction.CandidateID] {
			continue
		}
		if !known {
			metrics.ExpectedUnknowns++
			if prediction.CanonicalCreativeID == "" {
				metrics.CorrectResolvedUnknowns++
			} else {
				metrics.WrongResolvedUnknowns++
			}
			continue
		}
		if prediction.CanonicalCreativeID == expected && prediction.CanonicalCreativeID != "" {
			metrics.CorrectResolvedKnownIdentities++
		} else if prediction.CanonicalCreativeID == "" {
			metrics.MissedKnownIdentities++
		} else {
			metrics.WrongIdentityAssignments++
		}
	}
	return metrics
}
