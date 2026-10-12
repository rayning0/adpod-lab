package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rgan/adpod-lab/internal/fixtures"
)

func main() {
	seed := flag.Int64("seed", fixtures.Seed, "seed for deterministic fixture metadata")
	outputDir := flag.String("output", "data", "directory for generated fixtures")
	flag.Parse()

	creatives, candidates, groundTruth := fixtures.Generate(*seed)
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fail(err)
	}
	write := func(name string, value any) {
		file, err := os.Create(filepath.Join(*outputDir, name))
		if err != nil {
			fail(err)
		}
		defer file.Close()
		if err := fixtures.WriteJSON(file, value); err != nil {
			fail(err)
		}
	}
	write("creatives.json", creatives)
	write("candidates.json", candidates)
	write("ground_truth.json", groundTruth)
	fmt.Printf("generated %d creatives and %d candidates in %s (seed=%d)\n", len(creatives), len(candidates), *outputDir, *seed)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "generate fixtures: %v\n", err)
	os.Exit(1)
}
