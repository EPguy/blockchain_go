package blockchain

import (
	"github.com/boltdb/bolt"
)

const (
	BlocksBucket = "blocks"
	dbFile = "chain.db"
)

type BlockChain struct {
	LastHash []byte
	Database *bolt.DB
}

type BlockChainIterator struct {
	CurrentHash []byte
	Database *bolt.DB
}

func InitBlockChain() *BlockChain {
	var lastHash []byte

	db, err := bolt.Open(dbFile, 0600, nil)
	Handle(err)

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BlocksBucket))

		if b == nil {
			b, err := tx.CreateBucket([]byte(BlocksBucket))

			Handle(err)

			genesis := Genesis()
			err = b.Put(genesis.Hash, genesis.Serialize())

			Handle(err)

			err = b.Put([]byte("lh"), genesis.Hash)

			lastHash = genesis.Hash

			return err
		} else {
			lastHash = b.Get([]byte("lh"))

			return err
		}
	})

	Handle(err)

	blockChain := &BlockChain{lastHash, db}
	return blockChain
}

func (chain *BlockChain) AddBlock(data string) {
	newBlock := CreateBlock(data, chain.LastHash)
	
	err := chain.Database.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BlocksBucket))

		err := b.Put([]byte("lh"), newBlock.Hash)

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