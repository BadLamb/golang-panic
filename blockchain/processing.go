package blockchain

import (
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/dexm-coin/dexmd/util"
	"github.com/dexm-coin/dexmd/wallet"
	protobufs "github.com/dexm-coin/protobufs/build/blockchain"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

// Blockchain is an internal representation of a blockchain
type Blockchain struct {
	balancesDb    *leveldb.DB
	blockDb       *leveldb.DB
	ContractDb    *leveldb.DB
	StateDb       *leveldb.DB
	CasperVotesDb *leveldb.DB

	Mempool            *mempool
	TransactionArrived [][]byte

	Schnorr             map[string][]byte
	MTTrasaction        [][]byte
	MTReceipt           [][]byte
	RSchnorr            [][]byte
	PSchnorr            [][]byte
	MessagesTransaction [][]byte
	MessagesReceipt     [][]byte

	GenesisTimestamp uint64

	CurrentBlock      uint64
	CurrentCheckpoint uint64
	CurrentValidator  string
	CurrentVote       uint64
}

// BeaconChain is an internal representation of a beacon chain
type BeaconChain struct {
	MerkleRootsDb map[uint32]*leveldb.DB
	Validators    *ValidatorsBook

	CurrentBlock map[uint32]uint64
}

// NewBeaconChain create a new beacon chain
func NewBeaconChain(dbPath string) (*BeaconChain, error) {
	mrdb := make(map[uint32]*leveldb.DB)
	cb := make(map[uint32]uint64)

	for i := uint32(1); i < 11; i++ {
		db, err := leveldb.OpenFile(dbPath+".merkleroots"+strconv.Itoa(int(i)), nil)
		if err != nil {
			return nil, err
		}
		mrdb[i] = db
	}

	vd := NewValidatorsBook()
	return &BeaconChain{
		MerkleRootsDb: mrdb,
		Validators:    vd,
		CurrentBlock:  cb,
	}, nil
}

// NewBlockchain creates a database db
func NewBlockchain(dbPath string, index uint64) (*Blockchain, error) {
	db, err := leveldb.OpenFile(dbPath+".balances", nil)
	if err != nil {
		return nil, err
	}

	dbb, err := leveldb.OpenFile(dbPath+".blocks", nil)
	if err != nil {
		return nil, err
	}

	cdb, err := leveldb.OpenFile(dbPath+".code", nil)
	if err != nil {
		return nil, err
	}

	sdb, err := leveldb.OpenFile(dbPath+".memory", nil)
	if err != nil {
		return nil, err
	}

	cvdb, err := leveldb.OpenFile(dbPath+".votes", nil)
	if err != nil {
		return nil, err
	}

	// 1MB blocks
	mp := newMempool(1000000, 100)

	return &Blockchain{
		balancesDb:    db,
		blockDb:       dbb,
		ContractDb:    cdb,
		StateDb:       sdb,
		CasperVotesDb: cvdb,

		Mempool:            mp,
		TransactionArrived: [][]byte{},

		Schnorr:             make(map[string][]byte),
		MTTrasaction:        [][]byte{},
		MTReceipt:           [][]byte{},
		RSchnorr:            [][]byte{},
		PSchnorr:            [][]byte{},
		MessagesTransaction: [][]byte{},
		MessagesReceipt:     [][]byte{},

		CurrentBlock:      index,
		CurrentCheckpoint: 0,
		CurrentVote:       0,
	}, err
}

// GetWalletState returns the state of a wallet in the current block
func (bc *Blockchain) GetWalletState(wallet string) (protobufs.AccountState, error) {
	state := protobufs.AccountState{}
	raw, err := bc.balancesDb.Get([]byte(wallet), nil)
	if err != nil {
		return state, err
	}

	proto.Unmarshal(raw, &state)

	return state, nil
}

func (bc *BeaconChain) SaveMerkleRoots(mr *protobufs.MerkleRootsSigned) error {
	res, _ := proto.Marshal(mr)
	currShard := mr.GetShard()
	return bc.MerkleRootsDb[currShard].Put([]byte(strconv.Itoa(int(bc.CurrentBlock[currShard]))), res, nil)
}

func (bc *BeaconChain) GetMerkleRoots(index uint64, shard uint32) ([]byte, error) {
	return bc.MerkleRootsDb[shard].Get([]byte(strconv.Itoa(int(index))), nil)
}

// SaveBlockBeacon saves a block into the BeaconChain in a specific shard and index
func (bc *BeaconChain) SaveBlockBeacon(block *protobufs.MerkleRootsSigned, shard uint32, index int64) error {
	res, _ := proto.Marshal(block)
	return bc.MerkleRootsDb[shard].Put([]byte(strconv.Itoa(int(index))), res, nil)
}

// GetBlockBeacon returns the array of blocks at an index and at a specific shard
func (bc *BeaconChain) GetBlockBeacon(index int64, shard uint32) ([]byte, error) {
	return bc.MerkleRootsDb[shard].Get([]byte(strconv.Itoa(int(index))), nil)
}

// SaveBlock saves an unvalidated block into the blockchain to be used with Casper
func (bc *Blockchain) SaveBlock(block *protobufs.Block) error {
	res, _ := proto.Marshal(block)
	return bc.blockDb.Put([]byte(strconv.Itoa(int(block.GetIndex()))), res, nil)
}

// GetBlock returns the array of blocks at an index
func (bc *Blockchain) GetBlock(index uint64) ([]byte, error) {
	return bc.blockDb.Get([]byte(strconv.Itoa(int(index))), nil)
}

// GetContractCode returns the code of a contract at an address. Used
// as a wrapper so when we add diffed contracts in the future it's easier
// to change without breaking everything
func (bc *Blockchain) GetContractCode(address []byte) ([]byte, error) {
	return bc.ContractDb.Get(address, nil)
}

// ValidateBlock checks the validity of a block. It uses the current
// blockchain state so the passed block might become valid in the future.
// TODO Check validator
func (bc *Blockchain) ValidateBlock(block *protobufs.Block) (bool, error) {
	isTainted := make(map[string]bool)
	taintedState := make(map[string]protobufs.AccountState)

	// Genesis block is fine
	if block.GetIndex() == 0 {
		return true, nil
	}

	// TODO do a check that the signature of the block should match with the validator choosen for that index

	for i, t := range block.GetTransactions() {
		sender := wallet.BytesToAddress(t.GetSender(), t.GetShard())

		result, _ := proto.Marshal(t)

		valid, err := wallet.SignatureValid(t.GetSender(), t.GetR(), t.GetS(), result)
		if !valid || err != nil {
			log.Error("SignatureValid ", err)
			return false, err
		}

		balance := protobufs.AccountState{}

		// Check if the address state changed while processing this block
		// If it hasn't changed then pull the state from the blockchain, otherwise
		// get the updated copy instead
		if !isTainted[sender] {
			balance, err = bc.GetWalletState(sender)
			if err != nil {
				log.Error("getwalletstate ", err)
				return false, err
			}
		} else {
			balance = taintedState[sender]
		}

		// Check if balance is sufficient
		requiredBal, ok := util.AddU64O(t.GetAmount(), uint64(t.GetGas()))
		if requiredBal > balance.GetBalance() && ok {
			return false, errors.New("Balance is insufficient in transaction " + strconv.Itoa(i))
		}

		// Check if nonce is correct
		newNonce, ok := util.AddU32O(balance.GetNonce(), uint32(1))

		if t.GetNonce() != newNonce || !ok {
			return false, errors.New("Invalid nonce in transaction " + strconv.Itoa(i))
		}

		// Taint sender and update his balance. Reciver will be able to spend
		// his cash from the next block
		isTainted[sender] = true

		newBal, ok := util.SubU64O(balance.Balance, requiredBal)
		if !ok {
			return false, errors.New("Overflow in transaction " + strconv.Itoa(i))
		}

		balance.Balance = newBal
		balance.Nonce++
		taintedState[sender] = balance

		// To save a DB query we don't check the reciver for an overflow. If someone
		// gets that much cash we are gonna be fucked anyways because of PoS
	}

	return true, nil
}

func (bc *Blockchain) SetState(wallet string, newState *protobufs.AccountState) error {
	stateBytes, err := proto.Marshal(newState)
	if err != nil {
		return err
	}
	return bc.balancesDb.Put([]byte(wallet), stateBytes, nil)
}

// ValidateTransaction validates a transaction with the current state.
// Different from ValidateBlock because that has to verify for double spends
// inside the same block.
func (bc *Blockchain) ValidateTransaction(t *protobufs.Transaction) error {
	sender := wallet.BytesToAddress(t.GetSender(), t.GetShard())

	if !wallet.IsWalletValid(t.GetRecipient()) {
		return errors.New("Invalid recipient")
	}

	// TODO change []byte{} with the hash of the transaction
	valid, err := wallet.SignatureValid(t.GetSender(), t.GetR(), t.GetS(), []byte{})
	if !valid {
		return err
	}

	balance := protobufs.AccountState{}
	balance, err = bc.GetWalletState(sender)
	if err != nil {
		return err
	}

	if balance.Nonce != t.Nonce {
		return errors.New("Invalid nonce")
	}

	// Check if balance is sufficient
	requiredBal, ok := util.AddU64O(t.GetAmount(), uint64(t.GetGas()))
	if requiredBal > balance.GetBalance() && ok {
		return errors.New("Balance is insufficient in transaction")
	}

	// Check if nonce is correct
	if t.GetNonce() != balance.GetNonce() || !ok {
		return errors.New("Invalid nonce in transaction")
	}

	return nil
}

// GetNetworkIndex returns the current block index of the network
func (bc *Blockchain) GetNetworkIndex() int64 {
	timeSinceGenesis := time.Now().Unix() - int64(bc.GenesisTimestamp)

	index := math.Floor(float64(timeSinceGenesis) / 5.0)

	return int64(index)
}
