// To run benchmarks run 'go run tester.go dag.go pow.go'

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"
)

type BenchmarkConfig struct {
	N int
	C int
	R int
	D int
	p float64
}

func main() {
	configs := []BenchmarkConfig{
		{N: 4, C: 1, R: 1, D: 1, p: 0.5},
		{N: 6, C: 2, R: 2, D: 2, p: 0.75},
		{N: 8, C: 3, R: 3, D: 2, p: 1.0},
		{N: 10, C: 4, R: 3, D: 2, p: 0.8}, // More nodes
		{N: 6, C: 1, R: 3, D: 3, p: 0.5},  // Higher difficulty
		{N: 4, C: 2, R: 1, D: 1, p: 1.0},  // More corrupt nodes
	}

	file, err := os.Create("benchmark_results.csv")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"System", "N", "C", "R", "D", "p", "Time(s)", "TxSent", "TxConfirmed", "AvgConfidence", "Winner"})

	for _, cfg := range configs {
		// Simulate PoW
		fmt.Printf("Running PoW: N=%d C=%d R=%d D=%d p=%.2f\n", cfg.N, cfg.C, cfg.R, cfg.D, cfg.p)
		start := time.Now()
		txSent, txConfirmed, winner := SimulateBlockchainBenchmark(cfg.N, cfg.C, cfg.R, cfg.D, cfg.p)
		duration := float64(time.Since(start).Microseconds()) / 1000.0
		writer.Write([]string{
			"PoW",
			strconv.Itoa(cfg.N),
			strconv.Itoa(cfg.C),
			strconv.Itoa(cfg.R),
			strconv.Itoa(cfg.D),
			fmt.Sprintf("%.2f", cfg.p),
			fmt.Sprintf("%.2f", duration),
			strconv.Itoa(txSent),
			strconv.Itoa(txConfirmed),
			"",
			winner,
		})

		// Simulate DAG
		fmt.Printf("Running DAG: N=%d C=%d R=%d D=%d p=%.2f\n", cfg.N, cfg.C, cfg.R, cfg.D, cfg.p)
		start = time.Now()
		txSentDAG, txConfirmedDAG, avgConf := SimulateDAGBenchmark(cfg.N, cfg.C, cfg.R, cfg.D, cfg.p)
		duration = float64(time.Since(start).Microseconds()) / 1000.0
		writer.Write([]string{
			"DAG",
			strconv.Itoa(cfg.N),
			strconv.Itoa(cfg.C),
			strconv.Itoa(cfg.R),
			strconv.Itoa(cfg.D),
			fmt.Sprintf("%.2f", cfg.p),
			fmt.Sprintf("%.2f", duration),
			strconv.Itoa(txSentDAG),
			strconv.Itoa(txConfirmedDAG),
			fmt.Sprintf("%.2f", avgConf),
			"",
		})
		fmt.Println()
	}

	fmt.Println("Benchmarking complete. Results saved to benchmark_results.csv")
}
