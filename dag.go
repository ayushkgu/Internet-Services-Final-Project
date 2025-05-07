package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"
)

func computeHash(tx Transaction) string {
	data, _ := json.Marshal(tx.Sender + tx.Receiver + fmt.Sprintf("%f", tx.Amount) + strings.Join(tx.Parents, ""))
	record := fmt.Sprintf("%s%d", data, tx.Nonce)
	hash := sha256.Sum256([]byte(record))
	return fmt.Sprintf("%x", hash)
}

func mineTransaction(tx Transaction, difficulty int) Transaction {
	prefix := strings.Repeat("0", difficulty)
	for {
		tx.Hash = computeHash(tx)
		if strings.HasPrefix(tx.Hash, prefix) {
			break
		}
		tx.Nonce++
	}
	return tx
}

func createGenesis(difficulty int) []Transaction {
	gen := []Transaction{}
	for i := range 2 {
		tx := Transaction{
			Sender:   "genesis",
			Receiver: "network",
			Amount:   0.01 * float64(i+1),
			Parents:  []string{},
		}
		mineTransaction(tx, difficulty)
		gen = append(gen, tx)
	}
	return gen
}

func pickParents(Nodes []Transaction) []string {
	n := len(Nodes)
	i := rand.Intn(n)
	j := rand.Intn(n)
	for i == j {
		j = rand.Intn(n)
	}
	par := []string{Nodes[i].Hash, Nodes[j].Hash}
	return par
}

func isHonest(name string) bool {
	h := "honest"
	return len(name) > len(h) && name[0:len(h)] == h
}

func isCorrupt(name string) bool {
	c := "corrupt"
	return len(name) > len(c) && name[0:len(c)] == c
}

/*
	N = total number of nodes
	C = number of corrupt nodes
	R = number of rounds of transactions
	D = difficulty (NOTE: run time scales exponentially with difficulty)
	p = transaction reach {0 <= p <= 1} (i.e. p = 0.5 means each transaction reaches ~50% of nodes)
*/

func SimulateDAG(N, C, R, D int, p float64, verbose bool) (int, int, float64, int, int, int, int, float64, string, time.Duration, float64, float64) {
	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(N)

	// Note: DAG doesn't have Blocks, Transactions are the only object
	inboxes := make([]chan Transaction, N)
	receivers := make([]chan Transaction, N)
	var mu sync.Mutex
	var G = createGenesis(D)
	G1, G2 := G[0], G[1]
	G1.Hash = "gen1"
	G2.Hash = "gen2"

	/*
		NOTE:
		The current implementation allows for duplicate transactions in the final blockchain result
		The check for duplicate transactions is omitted in order to speed up the simulation
		However, the corrupt nodes have not been configured to take advantage of this
	*/
	transactionTracker := make(map[float64]int)
	transactionMap := make(map[float64]Transaction)

	for i := range N {
		inboxes[i] = make(chan Transaction)   // initialize each inbox
		receivers[i] = make(chan Transaction) // initialize each receiver
		go func() {
			defer wg.Done()

			HashMap := make(map[string]Transaction) // maps Hash to Transaction
			HashMap[G1.Hash] = G1
			HashMap[G2.Hash] = G2

			Nodes := []Transaction{}
			Nodes = append(Nodes, G1)
			Nodes = append(Nodes, G2)
			var exit = false
			transactions := []Transaction{} // unprocessed transactions

			for !exit {
				select {
				case t, ok := <-receivers[i]: // listen for mined transaction
					if ok {
						fmt.Println("here!!!")
						_, exists1 := HashMap[t.Parents[0]]
						_, exists2 := HashMap[t.Parents[1]]
						if exists1 && exists2 {
							HashMap[t.Hash] = t
							Nodes = append(Nodes, t)
						}
					}
				case t, ok := <-inboxes[i]: // read unmined transaction
					if ok {
						transactions = append(transactions, t)
					} else {
						exit = true
					}
				default: // mine transaction
					if len(transactions) > 0 {
						t := transactions[len(transactions)-1]
						transactions = transactions[:len(transactions)-1]
						t.Parents = pickParents(Nodes)
						t = mineTransaction(t, D)
						HashMap[t.Hash] = t
						Nodes = append(Nodes, t)

						l1 := getLabel(i, C)
						for j := range N { // broadcast transaction
							l2 := getLabel(j, C)
							if i == j || (l1 == "corrupt" && l2 == "honest") { // corrupt nodes only broadcast to other corrupt nodes
								continue
							}
							select {
							case receivers[j] <- t: // successfully sent
							default: // channel full or busy -- unable to send block
							}
						}
					}
				}
			}

			// Calculate Confidence Scores
			Confidence := make(map[string]int) // maps Hash to Number of Total References (Direct + Indirect)
			Tips := make(map[string]struct{})  // hashset to determine tips (unreferenced nodes)
			for key := range HashMap {
				Tips[key] = struct{}{}
			}

			for _, val := range HashMap {
				if len(val.Parents) < 2 {
					continue
				}
				p1 := val.Parents[0]
				p2 := val.Parents[1]
				_, exists1 := Tips[p1]
				_, exists2 := Tips[p2]
				if exists1 {
					delete(Tips, p1)
				}
				if exists2 {
					delete(Tips, p2)
				}
			}
			for tip := range Tips {
				visited := make(map[string]struct{})
				var dfs func(string)
				dfs = func(hash string) {
					if _, seen := visited[hash]; seen {
						return
					}
					visited[hash] = struct{}{}
					Confidence[hash]++ // Increment confidence for this transaction
					for _, parent := range HashMap[hash].Parents {
						dfs(parent)
					}
				}
				dfs(tip)
			}

			// Aggregate Results
			mu.Lock() // Apply lock to make sure multiple go routines don't simultaneously write to the map
			for k := range Confidence {
				_, exists := transactionTracker[HashMap[k].Amount]
				if !exists {
					transactionTracker[HashMap[k].Amount] = 0
					transactionMap[HashMap[k].Amount] = HashMap[k]
				}
				transactionTracker[HashMap[k].Amount] = transactionTracker[HashMap[k].Amount] + 1
			}
			mu.Unlock()
		}()
	}

	txSent := SendTransactions(N, C, R, inboxes, p) // same function from pow.go

	wg.Wait()

	corruptPercentage := getPercentage(C, N)
	duration := time.Since(start)

	// Output Results
	type kv struct {
		Key   Transaction
		Value int
	}

	var sortedConfidence []kv
	for k, v := range transactionTracker {
		sortedConfidence = append(sortedConfidence, kv{transactionMap[k], v})
	}

	sort.Slice(sortedConfidence, func(i, j int) bool {
		return sortedConfidence[i].Value > sortedConfidence[j].Value
	})

	totalConfidence := 0
	honestCount := 0
	corruptCount := 0
	honestConfidence := 0
	corruptConfidence := 0
	for _, kv := range sortedConfidence {
		totalConfidence += kv.Value
		if isHonest(kv.Key.Sender) {
			honestCount += 1
			honestConfidence += kv.Value
		} else if isCorrupt(kv.Key.Sender) {
			corruptCount += 1
			corruptConfidence += kv.Value
		}
	}

	avgConf_Honest := 0.0
	if honestCount > 0 {
		avgConf_Honest = float64(honestConfidence) / float64(honestCount)
	}
	avgConf_Corrupt := 0.0
	if corruptCount > 0 {
		avgConf_Corrupt = float64(corruptConfidence) / float64(corruptCount)
	}

	txConfirmed := 0
	avgConfidence := float64(totalConfidence) / float64(len(sortedConfidence))

	for _, kv := range sortedConfidence {
		if float64(kv.Value) >= avgConfidence {
			txConfirmed += 1
		}
	}

	txConfirmedPercentage := getPercentage(txConfirmed, txSent)
	winnerType := "honest"
	if avgConf_Corrupt > avgConf_Honest {
		winnerType = "corrupt"
	}

	// Print Result
	if verbose {
		fmt.Println("Transaction Confidence Levels:\n===============================")
		for _, kv := range sortedConfidence {
			fmt.Printf("Transaction: %s, Confidence: %d\n", formatTransaction(kv.Key), kv.Value)
		}

		fmt.Println("\nTotal nodes        =", N)
		fmt.Println("Corrupt nodes      =", C)
		fmt.Println("Corrupt %          =", corruptPercentage)
		fmt.Println("Rounds             =", R)
		fmt.Println("Difficulty         =", D)
		fmt.Println("txSent             =", txSent)
		fmt.Println("txConfirmed        =", txConfirmed)
		fmt.Println("txConfirmed %      =", txConfirmedPercentage)
		fmt.Println("Winner             =", winnerType)
		fmt.Printf("Duration (s)        = %.2f\n", duration.Seconds())
		fmt.Printf("avgConf_Honest      = %.2f\n", avgConf_Honest)
		fmt.Printf("avgConf_Corrupt     = %.2f\n", avgConf_Corrupt)
	}

	return N, C, corruptPercentage, R, D, txSent, txConfirmed, txConfirmedPercentage, winnerType, duration, avgConf_Honest, avgConf_Corrupt

}
