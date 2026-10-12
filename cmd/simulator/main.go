package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/rgan/adpod-lab/internal/creative"
	"github.com/rgan/adpod-lab/internal/model"
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

	fmt.Printf("AdPod Lab simulator (seed=%d): loaded %d creatives and %d candidates\n",
		*seed, len(creatives), len(candidates))
	fmt.Printf("identity resolution: %d universal, %d exact-media, %d fingerprint, %d unknown\n",
		methodCounts[string(creative.MethodUniversalID)],
		methodCounts[string(creative.MethodExactMedia)],
		methodCounts[string(creative.MethodFingerprint)],
		methodCounts[string(creative.MethodUnknown)]+methodCounts[string(creative.MethodConflict)])
	fmt.Printf("known-identity precision: %.2f, recall: %.2f (%d correct, %d missed/wrong)\n",
		metrics.Precision, metrics.Recall, metrics.TruePositives, metrics.FalseNegatives)
	fmt.Printf("expected-unknown handling: %d/%d correctly unresolved\n",
		metrics.CorrectUnknowns, metrics.ExpectedUnknowns)
}
