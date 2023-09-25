package abis

import (
	"github.com/urfave/cli/v2"
)

const (
	flagCategory = "BLOCKNATIVE DECODER"
)

var (
	FlagDB = &cli.StringFlag{
		Name:     "blocknative.decoder.db",
		Usage:    "Decoder DB connection string",
		Value:    "",
		Category: flagCategory,
	}

	Flags = []cli.Flag{FlagDB}
)
