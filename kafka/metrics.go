package kafka

import (
	"github.com/prometheus/client_golang/prometheus"
)

type ProducerMetrics struct {
	MessagesProduced *prometheus.CounterVec
}

type ConsumerMetrics struct {
	MessagesConsumed *prometheus.CounterVec
	BatchProcessTime *prometheus.HistogramVec
	ProcessingErrors *prometheus.CounterVec
	Errors           *prometheus.CounterVec
}

func NewProducerMetrics() *ProducerMetrics {
	return &ProducerMetrics{
		MessagesProduced: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kafka_produced_messages_total",
			Help: "Total number of produced messages",
		}, []string{"topic"}),
	}
}

func NewConsumerMetrics() *ConsumerMetrics {
	return &ConsumerMetrics{
		MessagesConsumed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kafka_consumed_messages_total",
			Help: "Total number of consumed messages",
		}, []string{"topic"}),
		BatchProcessTime: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kafka_batch_process_seconds",
			Help:    "Time taken to process batches",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 5),
		}, []string{"topic"}),
		ProcessingErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kafka_processing_errors_total",
			Help: "Total number of message processing errors",
		}, []string{"topic", "type"}),
		Errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kafka_consumer_errors_total",
			Help: "Total number of consumer errors",
		}, []string{"topic"}),
	}
}
