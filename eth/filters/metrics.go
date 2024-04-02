package filters

import (
	"github.com/ethereum/go-ethereum/metrics"
)

var (
	metricsPendingTxsNew          = metrics.NewRegisteredCounter("stream/pending_txs/new", nil)
	metricsPendingTxsEnd          = metrics.NewRegisteredCounter("stream/pending_txs/end", nil)
	metricsPendingTxsReceived     = metrics.NewRegisteredCounter("stream/pending_txs/received", nil)
	metricsPendingTxsGasTooLow    = metrics.NewRegisteredCounter("stream/pending_txs/gas_too_low", nil)
	metricsPendingTxsTraceSuccess = metrics.NewRegisteredCounter("stream/pending_txs/trace_success", nil)
	metricsPendingTxsTraceFailed  = metrics.NewRegisteredCounter("stream/pending_txs/trace_failed", nil)
	metricsPendingTxsSent         = metrics.NewRegisteredCounter("stream/pending_txs/sent", nil)

	metricsBlocksNew          = metrics.NewRegisteredCounter("stream/blocks/new", nil)
	metricsBlocksEnd          = metrics.NewRegisteredCounter("stream/blocks/end", nil)
	metricsBlocksReceived     = metrics.NewRegisteredCounter("stream/blocks/received", nil)
	metricsBlocksTraceSuccess = metrics.NewRegisteredCounter("stream/blocks/trace_success", nil)
	metricsBlocksTraceFailed  = metrics.NewRegisteredCounter("stream/blocks/trace_failed", nil)
	metricsBlocksSent         = metrics.NewRegisteredCounter("stream/blocks/sent", nil)

	metricsDroppedTxsNew      = metrics.NewRegisteredCounter("stream/dropped_txs/new", nil)
	metricsDroppedTxsEnd      = metrics.NewRegisteredCounter("stream/dropped_txs/end", nil)
	metricsDroppedTxsReceived = metrics.NewRegisteredCounter("stream/dropped_txs/received", nil)
	metricsDroppedTxsSent     = metrics.NewRegisteredCounter("stream/dropped_txs/sent", nil)

	metricsTracePendingTxTimer = metrics.NewRegisteredHistogram("stream/pending_txs/trace_duration", nil, metrics.NewExpDecaySample(1028, 0.015))
	metricsTraceBlockTimer     = metrics.NewRegisteredHistogram("stream/blocks/trace_duration", nil, metrics.NewExpDecaySample(1028, 0.015))
)
