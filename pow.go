package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"time"
)

type Block struct {
	Transactions []Transaction
	PrevHash     string
	Hash         string
	Nonce        int
}

type Transaction struct {
	Sender   string
	Receiver string
	Amount   float64
	// --  parameters below this are only used in DAG --
	Parents []string
	Hash    string
	Nonce   int
}

// --- Hashing and Mining ---
func calculateHash(block Block) string {
	blockData, _ := json.Marshal(block.Transactions)
	record := fmt.Sprintf("%s%s%d", blockData, block.PrevHash, block.Nonce)
	hash := sha256.Sum256([]byte(record))
	return fmt.Sprintf("%x", hash)
}

func mineBlock(block Block, difficulty int) Block {
	prefix := strings.Repeat("0", difficulty)
	for {
		block.Hash = calculateHash(block)
		if strings.HasPrefix(block.Hash, prefix) {
			break
		}
		block.Nonce++
	}
	return block
}

func createGenesisBlock(difficulty int) Block {
	block := Block{
		Transactions: []Transaction{},
		PrevHash:     "",
		Nonce:        0,
	}
	return mineBlock(block, difficulty)
}

func generateBlock(prev string, txs []Transaction, difficulty int) Block {
	block := Block{
		Transactions: txs,
		PrevHash:     prev,
	}
	return mineBlock(block, difficulty)
}

// --- Print Functions ---

func formatTransaction(tx Transaction) string {
	return fmt.Sprintf("%s → %s | Amount: %.2f", tx.Sender, tx.Receiver, tx.Amount)
}

func formatBlockHeader(b Block) string {
	return fmt.Sprintf("\n--- Block ---\nPrevHash: %s\nHash: %s", b.PrevHash, b.Hash)
}

func printBlockchain(Blockchain []Block) {
	fmt.Println("length =", len(Blockchain))
	for _, b := range Blockchain {
		fmt.Println(formatBlockHeader(b))
		for _, tx := range b.Transactions {
			fmt.Println(formatTransaction(tx))
		}
	}
}

// --- Blockchain Simulation ---

func getLabel(index, C int) string {
	if index < C {
		return "corrupt"
	}
	return "honest"
}
func getNum(index, C int) int {
	if index < C {
		return index + 1
	}
	return index + 1 - C
}

func SendTransactions(N, C, R int, inboxes []chan Transaction, p float64) (txSent int) {
	amt := 1.0
	for range R {
		honestTxs := []Transaction{}
		corruptTxs := []Transaction{}
		for i := range N {
			for j := range N {
				if i == j {
					continue
				}
				l1 := getLabel(i, C)
				l2 := getLabel(j, C)
				if l1 != l2 {
					continue
				}
				// Create transaction from node i to node j
				if rand.Float64() <= p { // p = probability of sending
					txSent += 1
					if l1 == "honest" {
						honestTxs = append(honestTxs, Transaction{
							Sender:   fmt.Sprintf("%s%d", l1, getNum(i, C)),
							Receiver: fmt.Sprintf("%s%d", l2, getNum(j, C)),
							Amount:   amt,
						})
					} else {
						corruptTxs = append(corruptTxs, Transaction{
							Sender:   fmt.Sprintf("%s%d", l1, getNum(i, C)),
							Receiver: fmt.Sprintf("%s%d", l2, getNum(j, C)),
							Amount:   amt,
						})
					}
				}
				// IMPORTANT: Each transaction is given a unique amount which serves as a unique identifier
				amt += 0.01
			}
		}

		// Send Transactions
		for i := range N {
			if i < C {
				for t := range corruptTxs {
					inboxes[i] <- corruptTxs[t]
				}
			} else {
				for t := range honestTxs {
					inboxes[i] <- honestTxs[t]
				}
			}
		}
	}

	// Close inbox after sending transactions
	for i := range N {
		close(inboxes[i])
	}

	return txSent
}

func buildBlockChain(HashMap map[string]Block, genesis Block, tail string) []Block {
	temp := []Block{}
	for {
		if HashMap[tail].Hash != "" {
			temp = append(temp, HashMap[tail])
		}
		next, ok := HashMap[tail]
		if !ok {
			break
		}
		tail = next.PrevHash
	}

	Blockchain := []Block{}
	Blockchain = append(Blockchain, genesis)
	for i := len(temp) - 1; i >= 0; i-- {
		Blockchain = append(Blockchain, temp[i])
	}
	return Blockchain
}

func countConfirmedTransactions(Blockchain []Block) int {
	txs := make(map[float64]struct{})
	for _, b := range Blockchain {
		for _, t := range b.Transactions {
			txs[t.Amount] = struct{}{}
		}
	}
	return len(txs)
}

func getPercentage(a, b int) float64 {
	percent := 100.0 * float64(a) / float64(b)
	rounded := math.Round(percent*100) / 100
	return rounded
}

/*
	N = total number of nodes
	C = number of corrupt nodes
	R = number of rounds of transactions
	D = difficulty (NOTE: run time scales exponentially with difficulty)
	p = transaction reach {0 <= p <= 1} (i.e. p = 0.5 means each transaction reaches ~50% of nodes)
*/

func SimulateBlockchain(N, C, R, D int, p float64, verbose bool) (int, int, float64, int, int, int, int, float64, string, time.Duration) {
	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(N)

	var blockWG sync.WaitGroup
	blockWG.Add(N)

	inboxes := make([]chan Transaction, N)
	receivers := make([]chan Block, N)
	var G = createGenesisBlock(D)

	var winner = []Block{}
	var winnerType = ""
	var winnerMu sync.Mutex

	for i := range N {
		inboxes[i] = make(chan Transaction) // initialize each inbox
		receivers[i] = make(chan Block, N)  // initialize each receiver
		go func(inbox chan Transaction, receiver chan Block, genesis Block) {
			defer wg.Done()
			/*
				NOTE:
				The current implementation allows for duplicate transactions in the final blockchain result
				The check for duplicate transactions is omitted in order to speed up the simulation
				However, the corrupt nodes have not been configured to take advantage of this
			*/
			HashMap := make(map[string]Block) // maps Hash to Block
			Counts := make(map[string]int)    // maps Hash to BlockChain length
			MaxLength := 0                    // track current max length
			MaxChain := ""                    // track the tail hash of the max length chain
			transactions := []Transaction{}   // unprocessed transactions
			var exit = false

			for !exit {
				select { // if a transaction and block are both available one is selected by Go (perhaps arbitrarily)
				case b, ok := <-receiver: // listen for blocks
					if ok {
						_, exists := HashMap[b.PrevHash]
						if exists {
							HashMap[b.Hash] = b
							Counts[b.Hash] = Counts[b.PrevHash] + 1
							if Counts[b.Hash] > MaxLength { // update max if needed
								MaxChain = b.Hash
								MaxLength = Counts[b.Hash]
							}
						} else {
							b.PrevHash = genesis.Hash
							HashMap[b.Hash] = b
							Counts[b.Hash] = 1
							if MaxChain == "" { // update max if this is the first chain
								MaxChain = b.Hash
								MaxLength = 1
							}
						}
					}
				case tx, ok := <-inbox: // read transactions
					if !ok {
						blockWG.Done()
						exit = true
					} else {
						transactions = append(transactions, tx)
					}
				default: // mine block
					if len(transactions) > 0 {
						var nextBlock = generateBlock(MaxChain, transactions, D)
						HashMap[nextBlock.Hash] = nextBlock
						Counts[nextBlock.Hash] = Counts[MaxChain] + 1
						MaxChain = nextBlock.Hash
						MaxLength = Counts[nextBlock.Hash]
						transactions = []Transaction{} // flush transactions

						l1 := getLabel(i, C)
						for j := range N { // broadcast block
							l2 := getLabel(j, C)
							if i == j || (l1 == "corrupt" && l2 == "honest") { // corrupt nodes only broadcast to other corrupt nodes
								continue
							}
							select {
							case receivers[j] <- nextBlock: // successfully sent
							default: // channel full or busy -- unable to send block
							}
						}
					}
				}
			}

			BlockChain := buildBlockChain(HashMap, G, MaxChain)

			// Apply lock to make sure multiple go routines don't simultaneously write to winner
			winnerMu.Lock()
			if len(BlockChain) > len(winner) {
				winner = BlockChain
				winnerType = getLabel(i, C)
			}
			winnerMu.Unlock()

		}(inboxes[i], receivers[i], G)
	}

	// Send transactions
	txSent := SendTransactions(N, C, R, inboxes, p)

	// Wait until all nodes are finished processing blocks before closing receivers
	blockWG.Wait()

	// Close receivers after processing all blocks
	for i := range N {
		close(receivers[i])
	}

	// Wait until all nodes are finished processing blocks before ending the simulation
	wg.Wait()

	corruptPercentage := getPercentage(C, N)
	txConfirmed := countConfirmedTransactions(winner)
	txConfirmedPercentage := getPercentage(txConfirmed, txSent)
	duration := time.Since(start)

	// Print Result
	if verbose {
		printBlockchain(winner)
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
	}

	return N, C, corruptPercentage, R, D, txSent, txConfirmed, txConfirmedPercentage, winnerType, duration
}
