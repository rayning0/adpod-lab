package pod

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rgan/adpod-lab/internal/model"
)

type fakeStore struct {
	next      int
	reserves  []string
	cancels   []string
	rejectIDs map[string]bool
	errorIDs  map[string]bool
}

func (s *fakeStore) Reserve(_ context.Context, _, creativeID string, _ int, _, _ time.Duration) (string, bool, error) {
	if s.rejectIDs[creativeID] {
		return "", false, nil
	}
	if s.errorIDs[creativeID] {
		return "", false, errors.New("store unavailable")
	}
	s.next++
	id := "reservation-" + string(rune('0'+s.next))
	s.reserves = append(s.reserves, creativeID)
	return id, true, nil
}

func (s *fakeStore) Confirm(context.Context, string, time.Time) error { return nil }

func (s *fakeStore) Cancel(_ context.Context, id string) error {
	s.cancels = append(s.cancels, id)
	return nil
}

func candidate(id string, cpm float64, replaceable bool) ResolvedCandidate {
	return ResolvedCandidate{
		Candidate: model.Candidate{
			CandidateID:     id,
			DurationSeconds: 30,
			CPM:             cpm,
			Replaceable:     replaceable,
		},
		Creative: model.ResolvedCreative{
			CandidateID:         id,
			CanonicalCreativeID: "creative-" + id,
		},
	}
}

func request() PodRequest {
	return PodRequest{HouseholdID: "household-1", DurationSeconds: 120}
}

func TestSelectHighestCPM(t *testing.T) {
	store := &fakeStore{}
	candidates := []ResolvedCandidate{
		candidate("d", 10, true), candidate("a", 40, true),
		candidate("c", 20, true), candidate("b", 30, true),
		candidate("e", 5, true),
	}

	result, err := (&Selector{FrequencyStore: store}).Select(request(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{result.Selected[0].Candidate.CandidateID, result.Selected[1].Candidate.CandidateID, result.Selected[2].Candidate.CandidateID, result.Selected[3].Candidate.CandidateID}
	want := []string{"a", "b", "c", "d"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("selected %v, want %v", got, want)
		}
	}
}

func TestRejectDuplicateCreative(t *testing.T) {
	store := &fakeStore{}
	first := candidate("a", 40, true)
	duplicate := candidate("b", 30, true)
	duplicate.Creative.CanonicalCreativeID = first.Creative.CanonicalCreativeID
	candidates := []ResolvedCandidate{first, duplicate, candidate("c", 20, true), candidate("d", 10, true), candidate("e", 1, true)}

	result, err := (&Selector{FrequencyStore: store}).Select(request(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	for _, selected := range result.Selected {
		if selected.Candidate.CandidateID == "b" {
			t.Fatal("duplicate creative was selected")
		}
	}
}

func TestPreserveProtectedInventory(t *testing.T) {
	store := &fakeStore{}
	protected := candidate("protected", 1, false)
	candidates := []ResolvedCandidate{candidate("a", 40, true), candidate("b", 30, true), candidate("c", 20, true), protected}
	req := request()
	req.ProtectedCandidate = &protected.Candidate

	result, err := (&Selector{FrequencyStore: store}).Select(req, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if result.Selected[0].Candidate.CandidateID != "protected" {
		t.Fatalf("first selected candidate = %q, want protected", result.Selected[0].Candidate.CandidateID)
	}
}

func TestProtectedDuplicate(t *testing.T) {
	store := &fakeStore{}
	first := candidate("protected-a", 1, false)
	second := candidate("protected-b", 2, false)
	second.Creative.CanonicalCreativeID = first.Creative.CanonicalCreativeID
	req := request()
	req.ProtectedCandidate = &first.Candidate

	result, err := (&Selector{FrequencyStore: store}).Select(req, []ResolvedCandidate{first, second, candidate("c", 20, true), candidate("d", 10, true)})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Selected) != 4 {
		t.Fatalf("selected %d candidates, want 4", len(result.Selected))
	}
	found := false
	for _, decision := range result.Decisions {
		if decision.CandidateID == "protected-b" && decision.Reason == "protected_duplicate_unavoidable" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing protected_duplicate_unavoidable decision")
	}
}

func TestFrequencyCapRejection(t *testing.T) {
	store := &fakeStore{rejectIDs: map[string]bool{"creative-a": true}}
	result, err := (&Selector{FrequencyStore: store}).Select(request(), []ResolvedCandidate{
		candidate("a", 40, true), candidate("b", 30, true), candidate("c", 20, true), candidate("d", 10, true), candidate("e", 1, true),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, selected := range result.Selected {
		if selected.Candidate.CandidateID == "a" {
			t.Fatal("frequency-capped candidate was selected")
		}
	}
}

func TestIncompletePodCancelsReservations(t *testing.T) {
	store := &fakeStore{}
	_, err := (&Selector{FrequencyStore: store}).Select(request(), []ResolvedCandidate{
		candidate("a", 40, true), candidate("b", 30, true), candidate("c", 20, true),
	})
	if err == nil {
		t.Fatal("expected incomplete pod error")
	}
	if len(store.cancels) != 3 {
		t.Fatalf("canceled %d reservations, want 3", len(store.cancels))
	}
}

func TestEqualCPMTiesUseCandidateID(t *testing.T) {
	store := &fakeStore{}
	result, err := (&Selector{FrequencyStore: store}).Select(request(), []ResolvedCandidate{
		candidate("d", 10, true), candidate("a", 10, true), candidate("c", 10, true), candidate("b", 10, true),
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"a", "b", "c", "d"} {
		if result.Selected[i].Candidate.CandidateID != id {
			t.Fatalf("slot %d = %q, want %q", i, result.Selected[i].Candidate.CandidateID, id)
		}
	}
}

func TestUnresolvedCandidatesUseDistinctReservationKeys(t *testing.T) {
	store := &fakeStore{}
	unknown := func(id string, cpm float64) ResolvedCandidate {
		c := candidate(id, cpm, true)
		c.Creative.CanonicalCreativeID = ""
		return c
	}
	_, err := (&Selector{FrequencyStore: store}).Select(request(), []ResolvedCandidate{
		unknown("a", 40), unknown("b", 30), unknown("c", 20), unknown("d", 10),
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"unknown:a", "unknown:b", "unknown:c", "unknown:d"} {
		if store.reserves[i] != want {
			t.Fatalf("reservation key %q, want %q", store.reserves[i], want)
		}
	}
}

func TestExampleExpectedOutput(t *testing.T) {
	store := &fakeStore{rejectIDs: map[string]bool{"creative-003": true}}
	makeExampleCandidate := func(id, creativeID string, cpm float64) ResolvedCandidate {
		c := candidate(id, cpm, true)
		c.Creative.CanonicalCreativeID = creativeID
		return c
	}
	candidates := []ResolvedCandidate{
		makeExampleCandidate("A", "creative-001", 30),
		makeExampleCandidate("B", "creative-001", 28),
		makeExampleCandidate("C", "creative-002", 25),
		makeExampleCandidate("D", "creative-003", 22),
		makeExampleCandidate("E", "creative-004", 20),
		makeExampleCandidate("F", "creative-005", 18),
	}

	result, err := (&Selector{FrequencyStore: store}).Select(request(), candidates)
	if err != nil {
		t.Fatal(err)
	}

	wantSelected := []string{"A", "C", "E", "F"}
	if len(result.Selected) != len(wantSelected) {
		t.Fatalf("selected %d candidates, want %d", len(result.Selected), len(wantSelected))
	}
	var totalDuration int
	var totalCPM float64
	for i, wantID := range wantSelected {
		selected := result.Selected[i]
		if selected.Candidate.CandidateID != wantID {
			t.Errorf("slot %d = %q, want %q", i+1, selected.Candidate.CandidateID, wantID)
		}
		totalDuration += selected.Candidate.DurationSeconds
		totalCPM += selected.Candidate.CPM
	}
	if totalDuration != 120 {
		t.Errorf("total duration = %d, want 120", totalDuration)
	}
	if totalCPM != 93 {
		t.Errorf("total CPM sum = %.2f, want 93", totalCPM)
	}
	if revenue := totalCPM / 1000; revenue != 0.093 {
		t.Errorf("estimated gross revenue = %.3f, want 0.093", revenue)
	}

	wantDecisions := []model.Decision{
		{CandidateID: "A", Accepted: true, Reason: "selected"},
		{CandidateID: "B", Accepted: false, Reason: "duplicate_creative"},
		{CandidateID: "C", Accepted: true, Reason: "selected"},
		{CandidateID: "D", Accepted: false, Reason: "frequency_cap"},
		{CandidateID: "E", Accepted: true, Reason: "selected"},
		{CandidateID: "F", Accepted: true, Reason: "selected"},
	}
	if len(result.Decisions) != len(wantDecisions) {
		t.Fatalf("decision count = %d, want %d", len(result.Decisions), len(wantDecisions))
	}
	for i, want := range wantDecisions {
		if result.Decisions[i] != want {
			t.Errorf("decision %d = %+v, want %+v", i, result.Decisions[i], want)
		}
	}
}
