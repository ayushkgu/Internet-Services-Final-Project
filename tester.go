package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"
)

/*
	terminal command to run main():
	"go run tester.go pow.go dag.go"
*/

type BenchmarkConfig struct {
	N int
	C int
	R int
	D int
	p float64
}

func main() {
	start := time.Now()

	tests := []BenchmarkConfig{
		{N: 10, C: 2, R: 3, D: 1, p: 0.8},
		{N: 10, C: 4, R: 3, D: 2, p: 0.8},
		{N: 15, C: 5, R: 2, D: 1, p: 0.6},
		{N: 15, C: 10, R: 2, D: 2, p: 0.6},
		{N: 20, C: 10, R: 2, D: 1, p: 0.4},
		{N: 20, C: 15, R: 2, D: 2, p: 0.4},
		{N: 25, C: 10, R: 1, D: 1, p: 0.2},
		{N: 25, C: 15, R: 1, D: 2, p: 0.2},
	}

	file, err := os.Create("benchmark_results.csv")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Headers
	writer.Write([]string{
		"Simulation Type",
		"Total Nodes",
		"Corrupt Nodes",
		"Corrupt %",
		"Rounds",
		"Broadcast Probability",
		"Difficulty",
		"txSent",
		"txConfirmed",
		"txConfirmed %",
		"Time (s)",
		"Winner",
		"avgConf_Honest",
		"avgConf_Corrupt",
	})

	num := 0
	fmt.Printf("Total Tests = %d\n", len(tests))
	for _, t := range tests {
		num += 1
		fmt.Printf("Running Test #%d: N=%d C=%d R=%d D=%d p=%.2f\n", num, t.N, t.C, t.R, t.D, t.p)

		// Test PoW
		N, C, corruptPercentage, R, D, txSent, txConfirmed, txConfirmedPercentage, winnerType, duration :=
			SimulateBlockchain(t.N, t.C, t.R, t.D, t.p, false)

		writer.Write([]string{
			"PoW",
			strconv.Itoa(N),
			strconv.Itoa(C),
			fmt.Sprintf("%.2f", corruptPercentage),
			strconv.Itoa(R),
			fmt.Sprintf("%.2f", t.p),
			strconv.Itoa(D),
			strconv.Itoa(txSent),
			strconv.Itoa(txConfirmed),
			fmt.Sprintf("%.2f", txConfirmedPercentage),
			fmt.Sprintf("%.2f", duration.Seconds()),
			winnerType,
		})

		// Test DAG
		avgConf_Honest := 0.0
		avgConf_Corrupt := 0.0

		N, C, corruptPercentage, R, D, txSent, txConfirmed, txConfirmedPercentage, winnerType, duration, avgConf_Honest, avgConf_Corrupt =
			SimulateDAG(t.N, t.C, t.R, t.D, t.p, false)

		writer.Write([]string{
			"DAG",
			strconv.Itoa(N),
			strconv.Itoa(C),
			fmt.Sprintf("%.2f", corruptPercentage),
			strconv.Itoa(R),
			fmt.Sprintf("%.2f", t.p),
			strconv.Itoa(D),
			strconv.Itoa(txSent),
			strconv.Itoa(txConfirmed),
			fmt.Sprintf("%.2f", txConfirmedPercentage),
			fmt.Sprintf("%.2f", duration.Seconds()),
			winnerType,
			fmt.Sprintf("%.2f", avgConf_Honest),
			fmt.Sprintf("%.2f", avgConf_Corrupt),
		})

	}

	duration := time.Since(start)
	fmt.Println("Total Test Time =", duration)
}
