package filters

import (
	"os"

	"github.com/prometheus/client_golang/prometheus"

	bnPrometheus "github.com/ethereum/go-ethereum/bn/prometheus"
	"github.com/ethereum/go-ethereum/log"
)

const streamSubsystem string = "stream"

var (
	metricsHostName string

	metricsPendingTxsNew = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "pending_txs_new",
			Help:      "Number of pending tx streams created",
		},
	)

	metricsPendingTxsEnd = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "pending_txs_end",
			Help:      "Number of pending tx streams ended",
		},
	)

	metricsPendingTxsReceived = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "pending_txs_received",
			Help:      "Number of pending txs received",
		},
	)

	metricsPendingTxsGasTooLow = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "pending_txs_gas_too_low",
			Help:      "Number txs ignore because of gas",
		},
	)

	metricsPendingTxsTraceSuccess = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "pending_txs_trace_success",
			Help:      "Number txs successfully traced",
		},
	)

	metricsPendingTxsTraceFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "pending_txs_trace_failed",
			Help:      "Number txs failed to trace",
		},
	)

	metricsPendingTxsSent = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "pending_txs_sent",
			Help:      "Number of pending txs sent",
		},
	)

	metricsBlocksNew = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "blocks_new",
			Help:      "Number of block streams created",
		},
	)

	metricsBlocksEnd = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "blocks_end",
			Help:      "Number of block streams ended",
		},
	)

	metricsBlocksReceived = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "blocks_received",
			Help:      "Number of blocks received",
		},
	)

	metricsBlocksTraceSuccess = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "blocks_trace_success",
			Help:      "Number of blocks successfully traced",
		},
	)

	metricsBlocksTraceFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "blocks_trace_failed",
			Help:      "Number of blocks failed to trace",
		},
	)

	metricsBlocksSent = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "blocks_sent",
			Help:      "Number of blocks sent",
		},
	)

	metricsDroppedTxsNew = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "dropped_txs_new",
			Help:      "Number of dropped tx streams created",
		},
	)

	metricsDroppedTxsEnd = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "dropped_txs_end",
			Help:      "Number of dropped tx streams ended",
		},
	)

	metricsDroppedTxsReceived = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "dropped_txs_received",
			Help:      "Number of dropped txs received",
		},
	)

	metricsDroppedTxsSent = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: streamSubsystem,
			Name:      "dropped_txs_sent",
			Help:      "Number of dropped txs sent",
		},
	)

	metricsTracePendingTxTimer = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: streamSubsystem,
			Name:      "trace_pending_tx_duration",
			Help:      "Trace pending tx duration in seconds",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"host"},
	)

	metricsTraceBlockTimer = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: streamSubsystem,
			Name:      "trace_blocks_duration",
			Help:      "Trace blocks duration in seconds",
			Buckets:   []float64{.01, .025, .05, .1, .25, .5, 1, 5, 10, 15},
		},
		[]string{"host"},
	)
)

func init() {
	var err error
	metricsHostName, err = os.Hostname()
	if err != nil {
		log.Error("failed to get hostname for metrics", "err", err)
	}

	register := func(c prometheus.Collector) {
		if err := bnPrometheus.Metrics.Register(c); err != nil {
			log.Error("failed to register metrics", "err", err)
		}
	}

	register(metricsPendingTxsNew)
	register(metricsPendingTxsEnd)
	register(metricsPendingTxsReceived)
	register(metricsPendingTxsGasTooLow)
	register(metricsPendingTxsTraceSuccess)
	register(metricsPendingTxsTraceFailed)
	register(metricsPendingTxsSent)

	register(metricsBlocksNew)
	register(metricsBlocksEnd)
	register(metricsBlocksReceived)
	register(metricsBlocksTraceSuccess)
	register(metricsBlocksTraceFailed)
	register(metricsBlocksSent)

	register(metricsDroppedTxsNew)
	register(metricsDroppedTxsEnd)
	register(metricsDroppedTxsReceived)
	register(metricsDroppedTxsSent)

	register(metricsTracePendingTxTimer)
	register(metricsTraceBlockTimer)
}
