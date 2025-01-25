package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	infuraWSS     = "wss://mainnet.infura.io/ws/v3/YOUR_PROJECT_ID"
	targetAddress = "0xYourWalletAddress"
	stateFile     = "last_block.txt" // ذخیره آخرین بلاک پردازش شده
	retryDelay    = 5 * time.Second
)

func main() {
	client, err := ethclient.Dial(infuraWSS)
	if err != nil {
		log.Fatalf("🚨 Connection failed: %v", err)
	}
	defer client.Close()

	// بارگذاری آخرین بلاک پردازش شده
	lastBlock := loadLastBlock()
	if lastBlock == nil {
		lastBlock = big.NewInt(0)
	}

	// ایجاد subscription برای بلاکهای جدید
	headers := make(chan *types.Header)
	sub, err := client.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		log.Fatalf("🚨 Subscription failed: %v", err)
	}

	log.Printf("👀 Monitoring for new transactions...")

	for {
		select {
		case err := <-sub.Err():
			log.Printf("⚠️ Subscription error: %v. Retrying...", err)
			time.Sleep(retryDelay)
			sub, err = retrySubscription(client)
			if err != nil {
				log.Printf("🔴 Failed to resubscribe: %v", err)
				continue
			}

		case header := <-headers:
			// پردازش هر بلاک جدید
			processNewBlock(client, header.Number, lastBlock)
			err := saveLastBlock(header.Number)
			if err != nil {
				log.Println("🔴 Failed to save last block")
			} // ذخیره آخرین بلاک
		}
	}
}

func processNewBlock(client *ethclient.Client, newBlock *big.Int, lastProcessed *big.Int) {
	if newBlock.Cmp(lastProcessed) <= 0 {
		return
	}

	query := ethereum.FilterQuery{
		FromBlock: newBlock,
		ToBlock:   newBlock,
		Addresses: []common.Address{common.HexToAddress(targetAddress)},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		log.Printf("🔴 Error fetching logs: %v", err)
		return
	}

	for _, l := range logs {
		fmt.Printf("🎉 New TX: %s | Block: %d\n", l.TxHash.Hex(), newBlock)
	}
}

// --- توابع کمکی ---
func loadLastBlock() *big.Int {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil
	}
	block := new(big.Int)
	block.SetString(string(data), 10)
	return block
}

func saveLastBlock(block *big.Int) error {
	err := os.WriteFile(stateFile, []byte(block.String()), 0644)
	if err != nil {
		return err
	}
	return nil
}

func retrySubscription(client *ethclient.Client) (ethereum.Subscription, error) {
	headers := make(chan *types.Header)
	return client.SubscribeNewHead(context.Background(), headers)
}
