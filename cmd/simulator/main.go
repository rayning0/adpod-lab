package main

import (
	"flag"
	"fmt"
)

func main() {
	seed := flag.Int64("seed", 20261011, "seed for the reproducible viewing simulation")
	flag.Parse()

	fmt.Printf("AdPod Lab simulator scaffold (seed=%d)\n", *seed)
}
