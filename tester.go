package main

import (
	"fmt"
	"time"
)

/*
	terminal command to run main():
	"go run tester.go pow.go dag.go"
*/

func main() {
	start := time.Now()

	// Choose which simulation to run by commenting out the other one
	SimulateBlockchain(5, 3, 1, 4, 0.5)
	// SimulateDAG(6, 2, 1, 2, 0.75)

	duration := time.Since(start)
	fmt.Printf("\nSimulation Time = %s\n", duration)
}
