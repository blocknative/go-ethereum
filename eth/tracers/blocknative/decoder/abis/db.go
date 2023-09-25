package abis

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

const (
	DefaultBlockchain = "ethereum"

	DefaultMaxOpenConns = 50
	DefaultMaxIdleConns = 10
	DefaultMaxConnLife  = 10
)

type Store interface {
	GetABIs(ctx context.Context) (abis []*ABI, err error)
	GetContracts(ctx context.Context, chainID *big.Int) (contracts []*Contract, err error)
	GetMethods(ctx context.Context) (methods []*Method, err error)
}

type PostgresStore struct {
	db *sql.DB
}

func (s *PostgresStore) Open(dbURL string) error {
	var err error
	s.db, err = sql.Open("postgres", dbURL)
	if err != nil {
		return err
	}

	s.db.SetMaxOpenConns(DefaultMaxOpenConns)
	s.db.SetMaxIdleConns(DefaultMaxIdleConns)
	s.db.SetConnMaxLifetime(DefaultMaxConnLife * time.Second)
	return nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) GetABIs(ctx context.Context) (abis []*ABI, err error) {
	rows, err := s.db.QueryContext(ctx, "SELECT hash_md5, contract_type, abi FROM abis")
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Error("error closing rows: " + err.Error())
		}
	}(rows)

	for rows.Next() {
		var (
			a       = &ABI{}
			hashHex string
			abiJSON []byte
		)
		if err = rows.Scan(&hashHex, &a.ContractType, &abiJSON); err != nil {
			return nil, err
		}

		a.HashMD5, err = DecodeMD5Hash([]byte(hashHex))
		if err != nil {
			return nil, err
		}

		abiObj, err := abi.JSON(bytes.NewReader(abiJSON))
		if err != nil {
			return nil, err
		}
		a.ABI = &abiObj

		abis = append(abis, a)
	}

	return abis, nil
}

func (s *PostgresStore) GetContracts(ctx context.Context, chainID *big.Int) (contracts []*Contract, err error) {
	network := chainIDToNetwork(chainID)
	rows, err := s.db.QueryContext(ctx, "SELECT hash_md5, address, contract_name FROM contracts WHERE blockchain = ? AND network = ?", DefaultBlockchain, network)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Error("error closing rows: " + err.Error())
		}
	}(rows)

	for rows.Next() {
		var (
			c            = &Contract{}
			hashHex      string
			address      string
			contractName sql.NullString
		)
		if err = rows.Scan(&hashHex, &address, &contractName); err != nil {
			return nil, err
		}

		c.HashMD5, err = DecodeMD5Hash([]byte(hashHex))
		if err != nil {
			return nil, err
		}

		c.Address = common.HexToAddress(address)
		c.ContractName = contractName.String
		contracts = append(contracts, c)
	}

	return nil, nil
}

func (s *PostgresStore) GetMethods(ctx context.Context) (methods []*Method, error error) {
	rows, err := s.db.QueryContext(ctx, "SELECT selector, signature, abi FROM methods")
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Error("error closing rows: " + err.Error())
		}
	}(rows)

	for rows.Next() {
		var (
			m       = &Method{}
			abiJSON []byte
		)
		if err = rows.Scan(&m.Selector, &m.Signature, &abiJSON); err != nil {
			return nil, err
		}

		abiObj, err := abi.JSON(bytes.NewReader(abiJSON))
		if err != nil {
			return nil, err
		}
		m.ABI = &abiObj

		methods = append(methods, m)
	}

	return nil, nil
}

func DecodeMD5Hash(b []byte) (MD5Hash, error) {
	b, err := hex.DecodeString(string(b))
	if err != nil {
		return MD5Hash{}, err
	}
	if len(b) != MD5HashSize {
		return MD5Hash{}, errors.New("invalid md5 hash size")
	}
	h := MD5Hash{}
	copy(h[:], b[:])
	return h, nil
}

func chainIDToNetwork(chainID *big.Int) string {
	switch chainID.Int64() {
	case 1:
		return "main"
	case 5:
		return "goerli"
	default:
		return ""
	}
}
