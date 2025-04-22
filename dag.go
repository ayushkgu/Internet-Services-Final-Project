package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
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

/*
	N = total number of nodes
	C = number of corrupt nodes
	R = number of rounds of transactions
	D = difficulty (NOTE: run time scales exponentially with difficulty)
	p = transaction reach {0 <= p <= 1} (i.e. p = 0.5 means each transaction reaches ~50% of nodes)
*/

func SimulateDAG(N, C, R, D int, p float64) {
	var wg sync.WaitGroup
	wg.Add(N)

	// Note: DAG doesn't have Blocks, Transactions are the only object
	inboxes := make([]chan Transaction, N)
	receivers := make([]chan Transaction, N)
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
			// fmt.Println(len(Tips), len(HashMap))
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
			for k := range Confidence {
				_, exists := transactionTracker[HashMap[k].Amount]
				if !exists {
					transactionTracker[HashMap[k].Amount] = 0
					transactionMap[HashMap[k].Amount] = HashMap[k]
				}
				transactionTracker[HashMap[k].Amount] = transactionTracker[HashMap[k].Amount] + 1
			}

		}()
	}

	SendTransactions(N, C, R, inboxes, p) // same function from pow.go

	wg.Wait()

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

	fmt.Println("Transaction Confidence Levels:\n===============================")
	for _, kv := range sortedConfidence {
		fmt.Printf("Transaction: %s, Confidence: %d\n", formatTransaction(kv.Key), kv.Value)
	}

}

// SimulateDAGBenchmark simulates a DAG-based blockchain benchmark
func SimulateDAGBenchmark(N, C, R, D int, p float64) (txSent int, txConfirmed int, avgConfidence float64) {
	var wg sync.WaitGroup
	wg.Add(N)

	inboxes := make([]chan Transaction, N)
	receivers := make([]chan Transaction, N)
	var G = createGenesis(D)
	G1, G2 := G[0], G[1]
	G1.Hash = "gen1"
	G2.Hash = "gen2"

	for i := 0; i < N; i++ {
		inboxes[i] = make(chan Transaction, 100)
		receivers[i] = make(chan Transaction, 100)
	}

	var mu sync.Mutex
	confidenceScores := []int{}
	transactionTracker := make(map[float64]bool)

	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			HashMap := map[string]Transaction{
				G1.Hash: G1,
				G2.Hash: G2,
			}
			Nodes := []Transaction{G1, G2}
			transactions := []Transaction{}
			exit := false

			for !exit {
				select {
				case t, ok := <-receivers[i]:
					if ok {
						_, ok1 := HashMap[t.Parents[0]]
						_, ok2 := HashMap[t.Parents[1]]
						if ok1 && ok2 {
							HashMap[t.Hash] = t
							Nodes = append(Nodes, t)
						}
					}
				case t, ok := <-inboxes[i]:
					if ok {
						transactions = append(transactions, t)
					} else {
						exit = true
					}
				default:
					if len(transactions) > 0 {
						t := transactions[len(transactions)-1]
						transactions = transactions[:len(transactions)-1]
						t.Parents = pickParents(Nodes)
						t = mineTransaction(t, D)
						HashMap[t.Hash] = t
						Nodes = append(Nodes, t)

						l1 := getLabel(i, C)
						for j := 0; j < N; j++ {
							l2 := getLabel(j, C)
							if i == j || (l1 == "corrupt" && l2 == "honest") {
								continue
							}
							select {
							case receivers[j] <- t:
							default:
							}
						}
					}
				}
			}

			Confidence := make(map[string]int)
			Tips := map[string]struct{}{}
			for h := range HashMap {
				Tips[h] = struct{}{}
			}
			for _, val := range HashMap {
				if len(val.Parents) < 2 {
					continue
				}
				delete(Tips, val.Parents[0])
				delete(Tips, val.Parents[1])
			}
			for tip := range Tips {
				visited := map[string]struct{}{}
				var dfs func(string)
				dfs = func(h string) {
					if _, seen := visited[h]; seen {
						return
					}
					visited[h] = struct{}{}
					Confidence[h]++
					for _, p := range HashMap[h].Parents {
						dfs(p)
					}
				}
				dfs(tip)
			}

			mu.Lock()
			for h, score := range Confidence {
				tx := HashMap[h]
				if !transactionTracker[tx.Amount] {
					txConfirmed++
					confidenceScores = append(confidenceScores, score)
					transactionTracker[tx.Amount] = true
				}
			}
			mu.Unlock()
		}(i)
	}

	// Count and send transactions
	amt := 1.0
	for round := 0; round < R; round++ {
		for i := 0; i < N; i++ {
			for j := 0; j < N; j++ {
				if i == j {
					continue
				}
				l1 := getLabel(i, C)
				l2 := getLabel(j, C)
				if l1 != l2 {
					continue
				}
				if rand.Float64() <= p {
					tx := Transaction{
						Sender:   fmt.Sprintf("%s%d", l1, getNum(i, C)),
						Receiver: fmt.Sprintf("%s%d", l2, getNum(j, C)),
						Amount:   amt,
					}
					inboxes[i] <- tx
					txSent++
					amt += 0.01
				}
			}
		}
	}
	for i := 0; i < N; i++ {
		close(inboxes[i])
	}

	wg.Wait()

	if len(confidenceScores) > 0 {
		sum := 0
		for _, s := range confidenceScores {
			sum += s
		}
		avgConfidence = float64(sum) / float64(len(confidenceScores))
	}

	return txSent, txConfirmed, avgConfidence
}
