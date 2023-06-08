package finch

import (
	bnPrometheus "github.com/ethereum/go-ethereum/bn/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

const finchBotSubsystem string = "finch"

var (
	receivedAMMPoolUpdateCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: finchBotSubsystem,
			Name:      "amm_pool_updates",
			Help:      "The number of AMM pool changes seen",
		},
	)

	receivedTradeTxCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: finchBotSubsystem,
			Name:      "received_trade_txs",
			Help:      "The number of trade transactions seen",
		},
	)

	receivedBotTxCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: finchBotSubsystem,
			Name:      "received_bot_txs",
			Help:      "The number of bot transactions seen",
		},
	)

	simulatedTradeTxCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: finchBotSubsystem,
			Name:      "simulated_trade_txs",
			Help:      "The number of trade transactions successfully simulated",
		},
	)

	ammPoolReserveChangesCheckedCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: finchBotSubsystem,
			Name:      "amm_pool_reserved_changes_checked",
			Help:      "The number of AMM liquidity pool changes checked",
		},
	)

	arbOpportunityFoundCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: finchBotSubsystem,
			Name:      "arb_opportunities_found",
			Help:      "The number of arb opportunities found",
		},
	)

	arbProfitFoundCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: finchBotSubsystem,
			Name:      "arb_total_profit_found",
			Help:      "The amount of arb profit found",
		},
	)
)

func init() {
	bnPrometheus.Metrics.Register(receivedAMMPoolUpdateCounter)
	bnPrometheus.Metrics.Register(receivedTradeTxCounter)
	bnPrometheus.Metrics.Register(receivedBotTxCounter)
	bnPrometheus.Metrics.Register(simulatedTradeTxCounter)
	bnPrometheus.Metrics.Register(ammPoolReserveChangesCheckedCounter)
	bnPrometheus.Metrics.Register(arbOpportunityFoundCounter)
	bnPrometheus.Metrics.Register(arbProfitFoundCounter)
}
