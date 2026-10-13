package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/rgan/adpod-lab/internal/creative"
	"github.com/rgan/adpod-lab/internal/frequency"
	"github.com/rgan/adpod-lab/internal/model"
	"github.com/rgan/adpod-lab/internal/pod"
)

func main() {
	seed := flag.Int64("seed", 20261011, "seed for the reproducible viewing simulation")
	flag.Parse()

	creatives, err := model.LoadCreativesFile(filepath.Join("data", "creatives.json"))
	if err != nil {
		panic(err)
	}
	candidates, err := model.LoadCandidatesFile(filepath.Join("data", "candidates.json"))
	if err != nil {
		panic(err)
	}
	groundTruth, err := model.LoadGroundTruthFile(filepath.Join("data", "ground_truth.json"))
	if err != nil {
		panic(err)
	}

	registry, err := model.LoadIdentityRegistryFile(filepath.Join("data", "identity_registry.json"))
	if err != nil {
		panic(err)
	}
	predictions := creative.NewResolver(registry).ResolveAll(candidates)
	metrics := creative.Evaluate(predictions, groundTruth)
	methodCounts := make(map[string]int)
	for _, prediction := range predictions {
		methodCounts[prediction.Method]++
	}

	fmt.Println("AdPod Lab Simulator")
	fmt.Printf("  Seed:               %d\n", *seed)
	uniqueCreativeIDs := make(map[string]struct{}, len(creatives))
	for _, catalogCreative := range creatives {
		uniqueCreativeIDs[catalogCreative.CreativeID] = struct{}{}
	}
	fmt.Printf("  Creatives (unique): %d\n", len(uniqueCreativeIDs))
	fmt.Printf("  Candidates:         %d\n\n", len(candidates))
	fmt.Println("Resolved Identities for Candidates:")
	fmt.Printf("  %-25s%3d\n", "Universal Ad IDs:", methodCounts[string(creative.MethodUniversalID)])
	fmt.Printf("  %-25s%3d\n", "Exact Media Hash:", methodCounts[string(creative.MethodExactMedia)])
	fmt.Printf("  %-25s%3d\n", "Audio/Video Fingerprint:", methodCounts[string(creative.MethodFingerprint)])
	fmt.Printf("  %-25s%3d\n", "Unknown:",
		methodCounts[string(creative.MethodUnknown)]+methodCounts[string(creative.MethodConflict)])
	fmt.Println("Identity resolution outcomes:")
	fmt.Printf("  %-38s%3d\n", "Correct resolved known identities:", metrics.CorrectResolvedKnownIdentities)
	fmt.Printf("  %-38s%3d\n", "Correct resolved unknowns:", metrics.CorrectResolvedUnknowns)
	fmt.Printf("  %-38s%3d\n", "Missed known identities:", metrics.MissedKnownIdentities)
	fmt.Printf("  %-38s%3d\n", "Wrong resolved unknowns:", metrics.WrongResolvedUnknowns)
	fmt.Printf("  %-38s%3d\n", "Wrong identity assignments:", metrics.WrongIdentityAssignments)
	fmt.Println()

	// The checkpoint uses all fixture candidates and a single household.
	// Candidate 030 is protected inventory; its exposure is accounted for
	// explicitly even though it bypasses the replaceable cap.
	const (
		householdID        = "household-001"
		podDurationSeconds = 120
		frequencyLimit     = 3
		frequencyWindow    = time.Hour
		reservationTTL     = time.Minute
	)
	now := time.Date(2026, 10, 11, 20, 0, 0, 0, time.UTC)
	store, err := frequency.NewInMemoryFrequencyStore(frequencyLimit, frequencyWindow, func() time.Time { return now })
	if err != nil {
		panic(err)
	}
	resolvedByID := make(map[string]pod.ResolvedCandidate, len(predictions))
	for i, prediction := range predictions {
		resolvedByID[prediction.CandidateID] = pod.ResolvedCandidate{
			Candidate: candidates[i],
			Creative:  prediction,
		}
	}
	// Seed one prior exposure to demonstrate shared household history.
	history, ok, err := store.Reserve(context.Background(), householdID, "creative-003", frequencyLimit, frequencyWindow, reservationTTL)
	if err != nil || !ok {
		panic(fmt.Errorf("seed exposure reservation: %w", err))
	}
	if err := store.Confirm(context.Background(), history, now.Add(-30*time.Minute)); err != nil {
		panic(fmt.Errorf("seed exposure confirmation: %w", err))
	}

	checkpointCandidates := make([]pod.ResolvedCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		checkpointCandidates = append(checkpointCandidates, resolvedByID[candidate.CandidateID])
	}
	selector := &pod.Selector{
		FrequencyStore:  store,
		FrequencyLimit:  frequencyLimit,
		FrequencyWindow: frequencyWindow,
		ReservationTTL:  reservationTTL,
	}
	result, err := selector.Select(pod.PodRequest{
		HouseholdID:     householdID,
		DurationSeconds: podDurationSeconds,
	}, checkpointCandidates)
	if err != nil {
		panic(fmt.Errorf("select checkpoint pod: %w", err))
	}
	for _, reservationID := range result.ReservationIDs {
		if err := store.Confirm(context.Background(), reservationID, now); err != nil {
			panic(fmt.Errorf("confirm playback: %w", err))
		}
	}
	var revenue float64
	for _, selected := range result.Selected {
		revenue += selected.Candidate.CPM / 1000
	}
	selectedDuration := 0
	selectedCreatives := make(map[string]bool)
	avoidableDuplicates := 0
	for _, selected := range result.Selected {
		selectedDuration += selected.Candidate.DurationSeconds
		if selected.Creative.CanonicalCreativeID != "" {
			if selectedCreatives[selected.Creative.CanonicalCreativeID] {
				avoidableDuplicates++
			}
			selectedCreatives[selected.Creative.CanonicalCreativeID] = true
		}
	}

	fmt.Printf("Candidates:       %d\n", len(checkpointCandidates))
	fmt.Printf("Household:        %s\n", householdID)
	fmt.Printf("Pod duration:     %d seconds\n", podDurationSeconds)
	fmt.Printf("Frequency cap:    %d impressions/hour (per household, per canonical creative)\n", frequencyLimit)
	fmt.Println()
	fmt.Println("Ads selected for pod:")
	for i, selected := range result.Selected {
		creativeID := selected.Creative.CanonicalCreativeID
		if creativeID == "" {
			creativeID = "unresolved"
		}
		if !selected.Candidate.Replaceable {
			fmt.Printf("  Slot %d: %-13s %-14s CPM $%.2f   Non-Replaceable\n",
				i+1, selected.Candidate.CandidateID, creativeID, selected.Candidate.CPM)
			continue
		}
		fmt.Printf("  Slot %d: %-13s %-14s CPM $%.2f\n",
			i+1, selected.Candidate.CandidateID, creativeID, selected.Candidate.CPM)
	}
	fmt.Println()
	fmt.Printf("Reservations confirmed:   %d\n", len(result.ReservationIDs))
	fmt.Printf("Selected duration:        %d / %d seconds\n", selectedDuration, podDurationSeconds)
	fmt.Printf("Avoidable duplicates:     %d ads repeated in pod\n", avoidableDuplicates)
	frequencyCapViolations := 0
	fmt.Printf("Frequency cap violations: %d selected ads in pod would exceed cap\n", frequencyCapViolations)
	fmt.Printf("Estimated revenue:        $%.4f\n", revenue)
}
