package blockchain

import (
	"encoding/hex"
	"fmt"
	"runtime"

	"github.com/boltdb/bolt"
)

const (
	BlocksBucket = "blocks"
	dbFile       = "chain.db"
	genesisData  = "First Transaction from Genesis"
)

type BlockChain struct {
	LastHash []byte
	Database *bolt.DB
}

type BlockChainIterator struct {
	CurrentHash []byte
	Database    *bolt.DB
}

func BucketExists(db *bolt.DB) bool {
	isExist := false
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BlocksBucket))

		lastHash := b.Get([]byte("lh"))

		if lastHash != nil {
			isExist = true
		}
		return nil
	})
	Handle(err)
	return isExist
}

func ContinueBlockChain(address string) *BlockChain {
	var lastHash []byte

	db, err := bolt.Open(dbFile, 0600, nil)
	Handle(err)

	if BucketExists(db) == false {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BlocksBucket))

		lastHash = b.Get([]byte("lh"))
		return nil
	})

	Handle(err)

	chain := &BlockChain{lastHash, db}

	return chain
}

func InitBlockChain(address string) *BlockChain {
	var lastHash []byte

	db, err := bolt.Open(dbFile, 0600, nil)
	Handle(err)

	if BucketExists(db) {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte(BlocksBucket))

		Handle(err)

		cbtx := CoinbaseTx(address, genesisData)

		genesis := Genesis(cbtx)
		fmt.Println("Genesis created")
		err = b.Put(genesis.Hash, genesis.Serialize())
		Handle(err)
		err = b.Put([]byte("lh"), genesis.Hash)

		lastHash = genesis.Hash

		return err
	})

	Handle(err)

	blockChain := &BlockChain{lastHash, db}
	return blockChain
}

func (chain *BlockChain) AddBlock(transactions []*Transaction) {
	newBlock := CreateBlock(transactions, chain.LastHash)

	err := chain.Database.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BlocksBucket))

		err := b.Put(newBlock.Hash, newBlock.Serialize())

		Handle(err)

		err = b.Put([]byte("lh"), newBlock.Hash)

		Handle(err)

		chain.LastHash = newBlock.Hash
		return nil
	})
	Handle(err)
}

func (chain *BlockChain) Iterator() *BlockChainIterator {
	iter := &BlockChainIterator{chain.LastHash, chain.Database}

	return iter
}

func (iter *BlockChainIterator) Next() *Block {
	var block *Block
	err := iter.Database.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BlocksBucket))

		encodedBlock := b.Get(iter.CurrentHash)
		block = Deserialize(encodedBlock)

		return nil
	})

	Handle(err)

	iter.CurrentHash = block.PrevHash

	return block
}

func (chain *BlockChain) FindUnspentTransactions(address string) []Transaction {
	var unspentTxs []Transaction

	spentTXOs := make(map[string][]int)

	iter := chain.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIdx, out := range tx.Outputs {
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}
				if out.CanBeUnlocked(address) {
					unspentTxs = append(unspentTxs, *tx)
				}
			}
			if tx.IsCoinBase() == false {
				for _, in := range tx.Inputs {
					if in.CanUnlock(address) {
						inTxID := hex.EncodeToString(in.ID)
						spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Out)
					}
				}
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}
	return unspentTxs
}

func (chain *BlockChain) FindUTXO(address string) []TxOutput {
	var UTXOs []TxOutput
	unspentTransactions := chain.FindUnspentTransactions(address)

	for _, tx := range unspentTransactions {
		for _, out := range tx.Outputs {
			if out.CanBeUnlocked(address) {
				UTXOs = append(UTXOs, out)
			}
		}
	}

	return UTXOs
}

func (chain *BlockChain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	unspentTxs := chain.FindUnspentTransactions(address)
	accumulated := 0

Work:
	for _, tx := range unspentTxs {
		txID := hex.EncodeToString(tx.ID)

		for outIdx, out := range tx.Outputs {
			if out.CanBeUnlocked(address) && accumulated < amount {
				accumulated += out.Value
				unspentOuts[txID] = append(unspentOuts[txID], outIdx)

				if accumulated >= amount {
					break Work
				}
			}
		}
	}
	return accumulated, unspentOuts
}
