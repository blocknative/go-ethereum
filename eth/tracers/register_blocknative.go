package tracers

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/eth/tracers/blocknative"
)

// Register a tracer lookup function that creates and returns blocknative
// tracers. We put the registration here instead of in the blocknative package
// so that it doesn't have to import this package, which imports geth/core, and
// in turn allows us to use the blocknative tracers from inside the geth/core
// package without causing circular dependency issues.
func init() {
	DefaultDirectory.Register("blocknative", blocknativeTracerCtor, false)

	// Also register the tracer under the old name for backwards compatibility.
	DefaultDirectory.Register("txnOpCodeTracer", blocknativeTracerCtor, false)
}

func blocknativeTracerCtor(_ *Context, cfg json.RawMessage) (*Tracer, error) {
	t, err := blocknative.NewTracerFromJSON(cfg)
	if err != nil {
		return nil, err
	}
	return &Tracer{
		Hooks:     t.Hooks(),
		GetResult: t.GetResult,
		Stop:      t.Stop,
	}, nil
}
