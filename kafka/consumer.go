package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

type Handler[T any] interface {
	HandleMessage(ctx context.Context, msg T) error
}

type BatchHandler[T any] interface {
	HandleBatch(ctx context.Context, messages []T) error
}

type Consumer[T any] struct {
	consumerGroup sarama.ConsumerGroup
	config        *KafkaConfig
	batchHandler  BatchHandler[T]
	singleHandler Handler[T]
	encoder       Encoder
	metrics       *ConsumerMetrics
	batchSize     int
	flushInterval time.Duration
}

func NewConsumer[T any](
	config *KafkaConfig,
	encoder Encoder,
	handler interface{},
) (*Consumer[T], error) {
	saramaConfig, err := config.SaramaConfig()
	if err != nil {
		return nil, err
	}

	group, err := sarama.NewConsumerGroup(config.Brokers, config.ConsumerGroup, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	c := &Consumer[T]{
		consumerGroup: group,
		config:        config,
		encoder:       encoder,
		metrics:       NewConsumerMetrics(),
		batchSize:     config.ConsumerSettings.BatchSize,
		flushInterval: config.ConsumerSettings.FlushInterval,
	}

	switch h := handler.(type) {
	case BatchHandler[T]:
		if config.ConsumerSettings.BatchSize <= 0 && config.ConsumerSettings.FlushInterval <= 0 {
			return nil, fmt.Errorf("batch processing requires BatchSize > 0 or FlushInterval > 0")
		}
		c.batchHandler = h
	case Handler[T]:
		c.singleHandler = h
	default:
		return nil, fmt.Errorf("invalid handler type")
	}

	return c, nil
}

func (c *Consumer[T]) Consume(ctx context.Context) error {
	handler := &consumerGroupHandler[T]{
		batchHandler:  c.batchHandler,
		singleHandler: c.singleHandler,
		ready:         make(chan bool),
		metrics:       c.metrics,
		encoder:       c.encoder,
		batchSize:     c.batchSize,
		flushInterval: c.flushInterval,
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := c.consumerGroup.Consume(ctx, c.config.Topics, handler); err != nil {
				return fmt.Errorf("consuming error: %w", err)
			}
		}
	}
}

type consumerGroupHandler[T any] struct {
	batchHandler  BatchHandler[T]
	singleHandler Handler[T]
	ready         chan bool
	metrics       *ConsumerMetrics
	encoder       Encoder
	batchSize     int
	flushInterval time.Duration
}

func (h *consumerGroupHandler[T]) Setup(sarama.ConsumerGroupSession) error {
	h.ready = make(chan bool)
	return nil
}

var once sync.Once

func (h *consumerGroupHandler[T]) Cleanup(sarama.ConsumerGroupSession) error {
	once.Do(func() { close(h.ready) })
	return nil
}

func (h *consumerGroupHandler[T]) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	batch := make([]*sarama.ConsumerMessage, 0, h.batchSize)

	// Initialize ticker only if flush interval is positive
	var tickerCh <-chan time.Time
	if h.flushInterval > 0 {
		ticker := time.NewTicker(h.flushInterval)
		defer ticker.Stop()
		tickerCh = ticker.C
	}

	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				// Handle remaining messages in batch before exit
				if len(batch) > 0 && h.batchHandler != nil {
					if err := h.processBatch(session.Context(), batch, session); err != nil {
						return err
					}
				}
				return nil
			}

			batch = append(batch, msg)
			h.metrics.MessagesConsumed.WithLabelValues(msg.Topic).Inc()

			if h.singleHandler != nil {
				if err := h.processSingleMessage(session.Context(), msg, session); err != nil {
					return err
				}
				batch = batch[:0] // Reset batch since we're processing individually
			} else if len(batch) >= h.batchSize {
				// Flush when batch size is reached
				if err := h.processBatch(session.Context(), batch, session); err != nil {
					return err
				}
				batch = batch[:0]
			}

		case <-tickerCh:
			// Flush on ticker interval (only if ticker is active)
			if len(batch) > 0 {
				if err := h.processBatch(session.Context(), batch, session); err != nil {
					return err
				}
				batch = batch[:0]
			}
		}
	}
}

func (h *consumerGroupHandler[T]) processSingleMessage(ctx context.Context, msg *sarama.ConsumerMessage, session sarama.ConsumerGroupSession) error {
	var decoded T
	if err := h.encoder.Decode(msg.Value, &decoded); err != nil {
		h.metrics.Errors.WithLabelValues(msg.Topic).Inc()
		session.MarkMessage(msg, "") // Mark as processed even if decoding fails
		return nil
	}

	return RetryOperation(ctx, 3, 100*time.Millisecond, func() error {
		err := h.singleHandler.HandleMessage(ctx, decoded)
		if err == nil {
			session.MarkMessage(msg, "")
		}
		return err
	})
}

func (h *consumerGroupHandler[T]) processBatch(ctx context.Context, batch []*sarama.ConsumerMessage, session sarama.ConsumerGroupSession) error {
	decodedMessages := make([]T, 0, len(batch))
	for _, msg := range batch {
		var decoded T
		if err := h.encoder.Decode(msg.Value, &decoded); err != nil {
			h.metrics.Errors.WithLabelValues(msg.Topic).Inc()
			continue // Skip invalid messages but process the rest
		}
		decodedMessages = append(decodedMessages, decoded)
	}

	if len(decodedMessages) == 0 {
		return nil
	}

	return RetryOperation(ctx, 3, 100*time.Millisecond, func() error {
		err := h.batchHandler.HandleBatch(ctx, decodedMessages)
		if err == nil {
			for _, msg := range batch {
				// Unmarked messages reprocessed In case of consumer crash during processing lead to duplicates
				session.MarkMessage(msg, "")
			}
		}
		return err
	})
}

func (c *Consumer[T]) Close() error {
	return c.consumerGroup.Close()
}
