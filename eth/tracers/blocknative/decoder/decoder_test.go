package decoder

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/tracers/blocknative/decoder/abis"
	"github.com/ethereum/go-ethereum/log"
)

func init() {
	db := &testDataStore{}
	if err := abis.LoadAndCacheAllFromStore(db, common.Big1); err != nil {
		log.Error("error loading data", "err", err.Error())
	}
}

// testDataStore implements the Store interface
type testDataStore struct{}

func (s *testDataStore) GetABIs(_ context.Context) ([]*abis.ABI, error) {
	file, err := os.Open("./testdata/abis.json")
	if err != nil {
		return nil, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Error("error closing file: " + err.Error())
		}
	}(file)

	fileContents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var testABIs []*abis.ABI
	if err = json.Unmarshal(fileContents, &testABIs); err != nil {
		return nil, err
	}
	return testABIs, nil
}

func (s *testDataStore) GetContracts(context.Context, *big.Int) (contracts []*abis.Contract, err error) {
	file, err := os.Open("./testdata/contracts.json")
	if err != nil {
		return nil, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Error("error closing file: " + err.Error())
		}
	}(file)

	fileContents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var testContracts []*Contract
	if err = json.Unmarshal(fileContents, &testContracts); err != nil {
		return nil, err
	}
	return testContracts, nil
}

func (s *testDataStore) GetMethods(_ context.Context) ([]*abis.Method, error) {
	return nil, nil
}
