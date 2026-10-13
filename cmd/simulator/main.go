package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/rgan/adpod-lab/internal/model"
	"github.com/rgan/adpod-lab/internal/simulator"
)

func main() {
	seed := flag.Int64("seed", 20261011, "seed for the reproducible viewing simulation")
	households := flag.Int("households", 20, "number of synthetic households")
	devices := flag.Int("devices", 3, "maximum devices per household")
	channels := flag.Int("channels", 5, "number of simulated channels")
	minutes := flag.Int("minutes", 120, "simulation duration in minutes")
	flag.Parse()

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
	report, err := simulator.Run(simulator.Config{
		Seed: *seed, Households: *households, MaxDevices: *devices,
		Channels: *channels, Minutes: *minutes, Candidates: candidates,
		Registry: registry, GroundTruth: groundTruth,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("AdPod Lab Simulator\n  Seed: %d  Households: %d  Devices: <=%d  Channels: %d  Minutes: %d\n  Candidates: %d  Events: %d\n\n",
		*seed, *households, *devices, *channels, *minutes, len(candidates), len(report.Events))
	printReport(report.Baseline, report.Optimized)
}

func printReport(baseline, optimized simulator.StrategyReport) {
	fmt.Printf("%-38s %14s %14s\n", "Metric", baseline.Name, optimized.Name)
	fmt.Printf("%-38s %14s %14s\n", "Processed breaks", formatInt(baseline.Metrics.ProcessedBreaks), formatInt(optimized.Metrics.ProcessedBreaks))
	fmt.Printf("%-38s %14s %14s\n", "Ad opportunities", formatInt(baseline.Metrics.AdOpportunities), formatInt(optimized.Metrics.AdOpportunities))
	fmt.Printf("%-38s %14s %14s\n", "Selected ads", formatInt(baseline.Metrics.SelectedAds), formatInt(optimized.Metrics.SelectedAds))
	fmt.Printf("%-38s %14s %14s\n", "Confirmed impressions", formatInt(baseline.Metrics.ConfirmedImpressions), formatInt(optimized.Metrics.ConfirmedImpressions))
	fmt.Printf("%-38s %14s %14s\n", "Canceled impressions", formatInt(baseline.Metrics.CanceledImpressions), formatInt(optimized.Metrics.CanceledImpressions))
	fmt.Printf("%-38s %14s %14s\n", "Delayed confirmations", formatInt(baseline.Metrics.DelayedConfirmations), formatInt(optimized.Metrics.DelayedConfirmations))
	fmt.Printf("%-38s %14s %14s\n", "Failed settlements (aggregate)", formatInt(baseline.Metrics.ExpiredOrUnknownSettlements), formatInt(optimized.Metrics.ExpiredOrUnknownSettlements))
	fmt.Printf("%-38s %14s %14s\n", "Incomplete pods", formatInt(baseline.Metrics.IncompletePods), formatInt(optimized.Metrics.IncompletePods))
	fmt.Printf("%-38s %14s %14s\n", "Intra-pod duplicates", formatInt(baseline.Metrics.IntraPodDuplicates), formatInt(optimized.Metrics.IntraPodDuplicates))
	fmt.Printf("%-38s %14s %14s\n", "Avoidable / protected duplicates", fmt.Sprintf("%d / %d", baseline.Metrics.AvoidableIntraPodDuplicates, baseline.Metrics.UnavoidableProtectedDuplicates), fmt.Sprintf("%d / %d", optimized.Metrics.AvoidableIntraPodDuplicates, optimized.Metrics.UnavoidableProtectedDuplicates))
	fmt.Printf("%-38s %14s %14s\n", "Frequency-cap violations", formatInt(baseline.Metrics.FrequencyCapViolations), formatInt(optimized.Metrics.FrequencyCapViolations))
	fmt.Printf("%-38s %14s %14s\n", "Estimated revenue", fmt.Sprintf("$%.4f", baseline.Metrics.EstimatedRevenue), fmt.Sprintf("$%.4f", optimized.Metrics.EstimatedRevenue))
	fmt.Printf("%-38s %14s %14s\n", "Confirmed revenue", fmt.Sprintf("$%.4f", baseline.Metrics.ConfirmedRevenue), fmt.Sprintf("$%.4f", optimized.Metrics.ConfirmedRevenue))
	fmt.Printf("%-38s %14s %14s\n", "Fill rate", fmt.Sprintf("%.2f%%", 100*baseline.Metrics.FillRate), fmt.Sprintf("%.2f%%", 100*optimized.Metrics.FillRate))
	fmt.Printf("%-38s %14s %14s\n", "Unresolved identities", formatInt(baseline.Metrics.UnresolvedIdentities), formatInt(optimized.Metrics.UnresolvedIdentities))
}

func formatInt(value int) string { return fmt.Sprintf("%d", value) }
