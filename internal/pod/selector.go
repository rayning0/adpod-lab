// Package pod assembles deterministic advertisement pods.
package pod

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/rgan/adpod-lab/internal/frequency"
	"github.com/rgan/adpod-lab/internal/model"
)

const (
	defaultFrequencyLimit = 1
	defaultWindow         = time.Hour
	defaultReservationTTL = 2 * time.Minute
)

// ResolvedCandidate is a source offer together with the result of creative
// identity resolution.
type ResolvedCandidate struct {
	Candidate model.Candidate
	Creative  model.ResolvedCreative
}

// PodRequest is the existing model request type, kept as an alias so callers
// can use the model package without converting request values.
type PodRequest = model.PodRequest

// SelectionResult contains the selected offers, explainability decisions, and
// the reservations created for the selected pod.
type SelectionResult struct {
	Selected       []ResolvedCandidate
	Decisions      []model.Decision
	ReservationIDs []string
}

// Selector greedily fills a pod with the highest-CPM eligible candidates.
type Selector struct {
	FrequencyStore  frequency.FrequencyStore
	FrequencyLimit  int
	FrequencyWindow time.Duration
	ReservationTTL  time.Duration
}

// Select reserves and selects a complete pod. If the requested duration
// cannot be filled, all reservations created by this attempt are canceled and
// an error is returned.
func (s *Selector) Select(request PodRequest, candidates []ResolvedCandidate) (SelectionResult, error) {
	if s == nil || s.FrequencyStore == nil {
		return SelectionResult{}, errors.New("pod selector requires a frequency store")
	}
	if request.HouseholdID == "" {
		return SelectionResult{}, errors.New("pod request requires a household ID")
	}
	if request.DurationSeconds <= 0 {
		return SelectionResult{}, errors.New("pod request requires a positive duration")
	}

	limit, window, ttl := s.settings()
	result := SelectionResult{}
	selectedCreatives := make(map[string]bool)
	reservations := make([]string, 0, len(candidates))
	remaining := request.DurationSeconds

	cancel := func(err error) (SelectionResult, error) {
		var cleanupErr error
		for _, id := range reservations {
			if cancelErr := s.FrequencyStore.Cancel(context.Background(), id); cancelErr != nil {
				cleanupErr = errors.Join(cleanupErr, cancelErr)
			}
		}
		return result, errors.Join(err, cleanupErr)
	}

	ordered := append([]ResolvedCandidate(nil), candidates...)
	protectedIDs := make(map[string]bool)
	if request.ProtectedCandidate != nil {
		protectedIDs[request.ProtectedCandidate.CandidateID] = true
	}
	for _, candidate := range ordered {
		if !candidate.Candidate.Replaceable {
			protectedIDs[candidate.Candidate.CandidateID] = true
		}
	}

	// Protected inventory is a fixed commitment and is selected before CPM
	// optimization. It is reserved with an effectively unlimited cap.
	for _, candidate := range ordered {
		if !protectedIDs[candidate.Candidate.CandidateID] {
			continue
		}
		if candidate.Candidate.DurationSeconds > remaining {
			return cancel(fmt.Errorf("protected candidate %q exceeds pod duration", candidate.Candidate.CandidateID))
		}
		reservationID, ok, err := s.FrequencyStore.Reserve(
			context.Background(), request.HouseholdID, reservationCreativeID(candidate),
			int(^uint(0)>>1), window, ttl,
		)
		if err != nil {
			return cancel(fmt.Errorf("reserve protected candidate %q: %w", candidate.Candidate.CandidateID, err))
		}
		if !ok {
			return cancel(fmt.Errorf("reserve protected candidate %q: frequency cap", candidate.Candidate.CandidateID))
		}
		reservations = append(reservations, reservationID)
		result.ReservationIDs = append(result.ReservationIDs, reservationID)
		result.Selected = append(result.Selected, candidate)
		result.Decisions = append(result.Decisions, model.Decision{
			CandidateID: candidate.Candidate.CandidateID,
			Accepted:    true,
			Reason:      "protected_selected",
		})
		if candidate.Creative.CanonicalCreativeID != "" {
			if selectedCreatives[candidate.Creative.CanonicalCreativeID] {
				result.Decisions[len(result.Decisions)-1].Reason = "protected_duplicate_unavoidable"
			}
			selectedCreatives[candidate.Creative.CanonicalCreativeID] = true
		}
		remaining -= candidate.Candidate.DurationSeconds
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Candidate.CPM != ordered[j].Candidate.CPM {
			return ordered[i].Candidate.CPM > ordered[j].Candidate.CPM
		}
		return ordered[i].Candidate.CandidateID < ordered[j].Candidate.CandidateID
	})

	for _, candidate := range ordered {
		if protectedIDs[candidate.Candidate.CandidateID] {
			continue
		}
		if remaining == 0 {
			break
		}
		if candidate.Candidate.DurationSeconds > remaining {
			result.Decisions = append(result.Decisions, reject(candidate, "duration_exceeded"))
			continue
		}
		creativeID := candidate.Creative.CanonicalCreativeID
		if creativeID != "" && selectedCreatives[creativeID] {
			result.Decisions = append(result.Decisions, reject(candidate, "duplicate_creative"))
			continue
		}

		reservationID, ok, err := s.FrequencyStore.Reserve(
			context.Background(), request.HouseholdID, reservationCreativeID(candidate),
			limit, window, ttl,
		)
		if err != nil {
			result.Decisions = append(result.Decisions, reject(candidate, "frequency_error"))
			continue
		}
		if !ok {
			result.Decisions = append(result.Decisions, reject(candidate, "frequency_cap"))
			continue
		}

		reservations = append(reservations, reservationID)
		result.ReservationIDs = append(result.ReservationIDs, reservationID)
		result.Selected = append(result.Selected, candidate)
		result.Decisions = append(result.Decisions, model.Decision{
			CandidateID: candidate.Candidate.CandidateID,
			Accepted:    true,
			Reason:      "selected",
		})
		if creativeID != "" {
			selectedCreatives[creativeID] = true
		}
		remaining -= candidate.Candidate.DurationSeconds
	}

	if remaining != 0 {
		return cancel(fmt.Errorf("incomplete pod: %d seconds unfilled", remaining))
	}
	return result, nil
}

func (s *Selector) settings() (int, time.Duration, time.Duration) {
	limit := s.FrequencyLimit
	if limit <= 0 {
		limit = defaultFrequencyLimit
	}
	window := s.FrequencyWindow
	if window <= 0 {
		window = defaultWindow
	}
	ttl := s.ReservationTTL
	if ttl <= 0 {
		ttl = defaultReservationTTL
	}
	return limit, window, ttl
}

func reservationCreativeID(candidate ResolvedCandidate) string {
	if candidate.Creative.CanonicalCreativeID != "" {
		return candidate.Creative.CanonicalCreativeID
	}
	// Unknown identities must not share an empty frequency key. CandidateID
	// isolates their reservations without claiming the ads are different.
	return "unknown:" + candidate.Candidate.CandidateID
}

func reject(candidate ResolvedCandidate, reason string) model.Decision {
	return model.Decision{CandidateID: candidate.Candidate.CandidateID, Accepted: false, Reason: reason}
}
