package decoder

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var (
	eventIDTransfer = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
	eventIDApproval = common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925")

	eventIDApprovalForAll = common.HexToHash("0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31")

	eventIDERC1155TransferSingle = common.HexToHash("0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62")
	eventIDERC1155TransferBatch  = common.HexToHash("0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb")
	eventIDERC1155URI            = common.HexToHash("0x0")

	eventIDToName = map[common.Hash]string{
		eventIDTransfer:              "Transfer",
		eventIDApproval:              "Approval",
		eventIDApprovalForAll:        "ApprovalForAll",
		eventIDERC1155TransferSingle: "TransferSingle",
		eventIDERC1155TransferBatch:  "TransferBatch",
		eventIDERC1155URI:            "URI",
	}
)

type Event struct {
	Address common.Address `json:"address"`
	ID      common.Hash    `json:"id"`
	Name    string         `json:"name,omitempty"`
	Topics  interface{}    `json:"topics"`
	Data    []interface{}  `json:"data,omitempty"`
}

type EventTransferTopics struct {
	From    common.Address `json:"from"`
	To      common.Address `json:"to"`
	TokenID *hexutil.Big   `json:"tokenID,omitempty"`
}

type EventApprovalTopics struct {
	Owner   common.Address `json:"owner"`
	Spender common.Address `json:"spender"`
	TokenID *hexutil.Big   `json:"tokenID,omitempty"`
}

type EventApprovalForAll struct {
	Owner    common.Address `json:"owner"`
	Operator common.Address `json:"operator"`

	Approved bool `json:"approved"`
}

type EventERC1155Transfer struct {
	Operator common.Address `json:"operator"`
	From     common.Address `json:"from"`
	To       common.Address `json:"to"`

	// ID    *big.Int `json:"id"`
	// Value *Amount  `json:"value"`
}

// type EventERC1155TransferBatch struct {
// 	Operator common.Address `json:"operator"`
// 	From     common.Address `json:"from"`
// 	To       common.Address `json:"to"`
//
// 	// IDs    []*big.Int `json:"ids"`
// 	// Values []*Amount  `json:"values"`
// }

// type EventERC1155ApprovalForAll struct {
// 	Owner    common.Address `json:"owner"`
// 	Operator common.Address `json:"operator"`
//
// 	Approved bool `json:"approved"`
// }
