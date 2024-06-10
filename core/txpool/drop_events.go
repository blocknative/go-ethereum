package txpool

const (
	DropUnderpriced = "underpriced-txs"
	DropLowNonce    = "low-nonce-txs"
	DropUnpayable   = "unpayable-txs"

	DropAccountCap      = "account-cap-txs" // Accounts exceeding txpool.accountslots transactions
	DropReplaced        = "replaced-txs"
	DropUnexecutable    = "unexecutable-txs"
	DropTruncating      = "truncating-txs"
	DropOld             = "old-txs"
	DropGasPriceUpdated = "updated-gas-price"
)
