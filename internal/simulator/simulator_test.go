package simulator

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/rgan/adpod-lab/internal/fixtures"
	"github.com/rgan/adpod-lab/internal/frequency"
	"github.com/rgan/adpod-lab/internal/model"
	"github.com/rgan/adpod-lab/internal/pod"
)

func testConfig() Config {
	_, candidates, registry, groundTruth := fixtures.Generate(DefaultSeed)
	return Config{
		Seed: DefaultSeed, Households: 20, MaxDevices: 3, Channels: 5,
		Minutes: 120, Candidates: candidates, Registry: registry, GroundTruth: groundTruth,
	}
}

func TestDeterminism(t *testing.T) {
	first, err := Run(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	second, err := Run(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same seed and configuration produced different events or metrics")
	}
}

func TestSeedVariationChangesViewingEvents(t *testing.T) {
	first := GenerateEvents(Config{Seed: 1, Households: 4, MaxDevices: 3, Channels: 5, Minutes: 60})
	second := GenerateEvents(Config{Seed: 2, Households: 4, MaxDevices: 3, Channels: 5, Minutes: 60})
	if reflect.DeepEqual(first, second) {
		t.Fatal("different seeds produced identical viewing events")
	}
}

func TestEventsAreChronologicalWithExplicitPriority(t *testing.T) {
	events := GenerateEvents(Config{Seed: 7, Households: 3, MaxDevices: 2, Channels: 5, Minutes: 60})
	for i := 1; i < len(events); i++ {
		previous, current := events[i-1], events[i]
		if current.Timestamp.Before(previous.Timestamp) {
			t.Fatalf("event %d is earlier than event %d", i, i-1)
		}
		if current.Timestamp.Equal(previous.Timestamp) &&
			eventPriority(current.EventType) < eventPriority(previous.EventType) {
			t.Fatalf("event priority is not stable at %s", current.Timestamp)
		}
		if current.EventType != EventAdBreak {
			continue
		}
	}
	for _, event := range events {
		if event.EventType == EventPlaybackConfirm || event.EventType == EventPlaybackCancel || event.EventType == EventPlaybackDelayed {
			if event.Timestamp.IsZero() || event.OpportunityID == "" {
				t.Fatalf("playback event lacks replay information: %+v", event)
			}
		}
	}
}

func TestReplayUsesSeparateStoresAndSharedEvents(t *testing.T) {
	report, err := Run(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Events) == 0 || report.Baseline.Metrics.AdOpportunities != report.Optimized.Metrics.AdOpportunities {
		t.Fatalf("strategies did not receive the same event opportunities: %+v", report)
	}
	again, err := Run(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if report.Baseline != again.Baseline || report.Optimized != again.Optimized {
		t.Fatal("replay state leaked between runs")
	}
}

func TestReplayProcessesCanceledAndDelayedOutcomes(t *testing.T) {
	report, err := Run(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, strategy := range []StrategyReport{report.Baseline, report.Optimized} {
		if strategy.Metrics.CanceledImpressions == 0 {
			t.Fatalf("%s did not process any cancellation", strategy.Name)
		}
		if strategy.Metrics.DelayedConfirmations == 0 {
			t.Fatalf("%s did not process any successful delayed confirmation", strategy.Name)
		}
		if strategy.Metrics.ExpiredOrUnknownSettlements == 0 {
			t.Fatalf("%s did not exercise an expired delayed reservation", strategy.Name)
		}
	}
}

func TestHouseholdScopeSharesCapAcrossDevices(t *testing.T) {
	now := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	store, err := frequency.NewInMemoryFrequencyStore(1, time.Hour, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	id, ok, err := store.Reserve(context.Background(), "household-001", "creative-001", 1, time.Hour, time.Minute)
	if err != nil || !ok {
		t.Fatalf("initial reservation = %q, %t, %v", id, ok, err)
	}
	if err := store.Confirm(context.Background(), id, now); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.Reserve(context.Background(), "household-001", "creative-001", 1, time.Hour, time.Minute); err != nil || ok {
		t.Fatal("second device could reserve a household-capped creative")
	}
	if _, ok, err := store.Reserve(context.Background(), "device-002", "creative-001", 1, time.Hour, time.Minute); err != nil || !ok {
		t.Fatal("device-scoped baseline unexpectedly shared household state")
	}
}

func TestCanonicalCrossSourceDeduplication(t *testing.T) {
	candidates := make([]pod.ResolvedCandidate, 5)
	for i := range candidates {
		candidates[i] = pod.ResolvedCandidate{
			Candidate: model.Candidate{
				CandidateID:      "candidate-" + string(rune('a'+i)),
				Source:           "source-" + string(rune('a'+i)),
				SourceCreativeID: "local-" + string(rune('a'+i)),
				DurationSeconds:  30, CPM: float64(40 - i), Replaceable: true,
			},
			Creative: model.ResolvedCreative{CandidateID: "candidate-" + string(rune('a'+i)), CanonicalCreativeID: "creative-001"},
		}
	}
	candidates[2].Creative.CanonicalCreativeID = "creative-002"
	candidates[3].Creative.CanonicalCreativeID = "creative-003"
	candidates[4].Creative.CanonicalCreativeID = "creative-004"
	store, _ := frequency.NewInMemoryFrequencyStore(10, time.Hour, time.Now)
	optimized, err := (&pod.Selector{FrequencyStore: store, DedupeCanonical: true}).Select(
		pod.PodRequest{HouseholdID: "household-001", DurationSeconds: 120}, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(optimized.Selected) != 4 {
		t.Fatalf("optimized selected %d ads, want 4 after rejecting the cross-source duplicate", len(optimized.Selected))
	}
}

func TestCapEvaluationCountsEachExcessOnce(t *testing.T) {
	base := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	expectations := model.ResolutionExpectations{Resolved: map[string]string{
		"a": "creative-001", "b": "creative-001", "c": "creative-001", "d": "creative-001",
	}}
	impressions := []confirmedAd{
		{base, "household-001", model.Candidate{CandidateID: "a"}},
		{base.Add(10 * time.Minute), "household-001", model.Candidate{CandidateID: "b"}},
		{base.Add(20 * time.Minute), "household-001", model.Candidate{CandidateID: "c"}},
		{base.Add(30 * time.Minute), "household-001", model.Candidate{CandidateID: "d"}},
	}
	if got := countCapViolations(impressions, expectations); got != 1 {
		t.Fatalf("cap violations = %d, want 1", got)
	}
}

func TestGroundTruthDuplicateEvaluationIgnoresResolverMisses(t *testing.T) {
	report := Metrics{}
	selected := []pod.ResolvedCandidate{
		{
			Candidate: model.Candidate{CandidateID: "source-a"},
			Creative:  model.ResolvedCreative{CandidateID: "source-a", CanonicalCreativeID: "wrong-a"},
		},
		{
			Candidate: model.Candidate{CandidateID: "source-b"},
			Creative:  model.ResolvedCreative{CandidateID: "source-b"},
		},
	}
	groundTruth := model.ResolutionExpectations{Resolved: map[string]string{
		"source-a": "creative-001",
		"source-b": "creative-001",
	}}
	countPodDuplicates(&report, selected, groundTruth)
	if report.IntraPodDuplicates != 1 {
		t.Fatalf("ground-truth duplicates = %d, want 1", report.IntraPodDuplicates)
	}
}

func TestDelayedConfirmationAfterTTLDoesNotCreateRevenue(t *testing.T) {
	now := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	store, err := frequency.NewInMemoryFrequencyStore(1, time.Hour, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	reservation, ok, err := store.Reserve(context.Background(), "household-001", "creative-001", 1, time.Hour, 5*time.Minute)
	if err != nil || !ok {
		t.Fatalf("reserve = %q, %t, %v", reservation, ok, err)
	}
	now = now.Add(5 * time.Minute)
	if err := store.Confirm(context.Background(), reservation, now); err != frequency.ErrUnknownReservation {
		t.Fatalf("late confirmation error = %v, want ErrUnknownReservation", err)
	}
	if _, ok, err := store.Reserve(context.Background(), "household-001", "creative-001", 1, time.Hour, time.Minute); err != nil || !ok {
		t.Fatalf("expired reservation did not release capacity: ok=%t err=%v", ok, err)
	}
}

func TestRevenueAccountingSeparatesSelectedAndConfirmed(t *testing.T) {
	report, err := Run(Config{
		Seed: 9, Households: 2, MaxDevices: 1, Channels: 2, Minutes: 45,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, strategy := range []StrategyReport{report.Baseline, report.Optimized} {
		if strategy.Metrics.SelectedAds == 0 || strategy.Metrics.EstimatedRevenue <= 0 {
			t.Fatalf("%s has no selected revenue", strategy.Name)
		}
		if strategy.Metrics.ConfirmedImpressions > strategy.Metrics.SelectedAds {
			t.Fatalf("%s confirmed more ads than it selected", strategy.Name)
		}
		if strategy.Metrics.ConfirmedRevenue > strategy.Metrics.EstimatedRevenue {
			t.Fatalf("%s confirmed revenue exceeds estimated selected revenue", strategy.Name)
		}
	}
}

func TestFullIntegrationUsesAllThirtyCandidates(t *testing.T) {
	cfg := testConfig()
	if len(cfg.Candidates) != 30 {
		t.Fatalf("fixture candidates = %d, want 30", len(cfg.Candidates))
	}
	report, err := Run(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if report.Baseline.Metrics.ProcessedBreaks == 0 || report.Optimized.Metrics.ProcessedBreaks == 0 {
		t.Fatal("full integration produced no opportunities")
	}
	if report.Baseline.Metrics.UnresolvedIdentities != 3 || report.Optimized.Metrics.UnresolvedIdentities != 3 {
		t.Fatal("evaluation omitted the three unresolved fixture identities")
	}
}
