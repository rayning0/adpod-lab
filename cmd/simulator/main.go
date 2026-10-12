package main

import (
	"flag"
	"fmt"
	"path/filepath"

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
	fmt.Printf("AdPod Lab simulator (seed=%d): loaded %d creatives and %d candidates\n", *seed, len(creatives), len(candidates))
}
