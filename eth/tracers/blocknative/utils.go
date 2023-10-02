package blocknative

import (
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

// Big overrides hexutil.Big to to use a faster serialization method.
type Big hexutil.Big

// MarshalText implements encoding.TextMarshaler
func (b Big) MarshalText() ([]byte, error) {
	b2 := big.Int(b)
	return []byte("0x" + b2.Text(16)), nil
}

func (b Big) UnmarshalJSON(input []byte) error {
	b2 := hexutil.Big(b)
	return b2.UnmarshalJSON(input)
}

// Uint64 overrides hexutil.Uint64 to to use a faster serialization method.
type Uint64 hexutil.Uint64

// MarshalText implements encoding.TextMarshaler.
func (b Uint64) MarshalText() ([]byte, error) {
	return []byte("0x" + strconv.FormatUint(uint64(b), 16)), nil
}

func (b *Uint64) UnmarshalJSON(input []byte) error {
	u := hexutil.Uint64(*b)
	if err := u.UnmarshalJSON(input); err != nil {
		return err
	}
	*b = Uint64(u)
	return nil
}
