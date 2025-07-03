package kafka

import (
	"context"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
)

// TestEvent represents a sample event for testing
type TestEvent struct {
	ID      string `json:"id"`
	Data    string `json:"data"`
	Version int    `json:"version"`
}

// SingleMessageHandler implements Handler interface for testing
type SingleMessageHandler struct {
	messages []TestEvent
	mu       sync.Mutex
}

func (h *SingleMessageHandler) HandleMessage(ctx context.Context, msg TestEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, msg)
	return nil
}

func (h *SingleMessageHandler) GetMessages() []TestEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.messages
}

// BatchMessageHandler implements BatchHandler interface for testing
type BatchMessageHandler struct {
	batches [][]TestEvent
	mu      sync.Mutex
}

func (h *BatchMessageHandler) HandleBatch(ctx context.Context, messages []TestEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.batches = append(h.batches, messages)
	return nil
}

func (h *BatchMessageHandler) GetBatches() [][]TestEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.batches
}

// FailingHandler implements Handler interface for testing retries
type FailingHandler struct {
	attempts int
	maxFails int
}

func (h *FailingHandler) HandleMessage(_ context.Context, _ TestEvent) error {
	h.attempts++
	if h.attempts <= h.maxFails {
		return fmt.Errorf("failing attempt %d", h.attempts)
	}
	return nil
}

var (
	brokers []string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	kafkaContainer, err := kafka.Run(ctx, "confluentinc/confluent-local:7.5.0")
	if err != nil {
		log.Fatalf("Failed to start Kafka container: %v", err)
	}

	brokers, err = kafkaContainer.Brokers(ctx)
	if err != nil {
		log.Fatalf("Failed to get brokers: %v", err)
	}

	m.Run()

	if err := kafkaContainer.Terminate(ctx); err != nil {
		log.Fatalf("Failed to terminate container: %v", err)
	}
}

func TestProducerSyncMessages(t *testing.T) {
	config := DefaultConfig()
	config.Brokers = brokers
	config.Topics = []string{"test-sync-producer"}

	producer, err := NewProducer(config, JSONEncoder{}, ProducerOptions{})
	require.NoError(t, err)
	defer producer.Close()

	testCases := []struct {
		name    string
		key     string
		event   TestEvent
		headers map[string]string
		wantErr bool
	}{
		{
			name:  "basic message",
			key:   "key1",
			event: TestEvent{ID: "1", Data: "test1", Version: 1},
		},
		{
			name:  "message with headers",
			key:   "key2",
			event: TestEvent{ID: "2", Data: "test2", Version: 1},
			headers: map[string]string{
				"trace-id": "abc-123",
				"source":   "test",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := producer.SendMessage(config.Topics[0], tc.key, tc.event, tc.headers)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProducerAsyncMessages(t *testing.T) {
	config := DefaultConfig()
	config.Brokers = brokers
	config.Topics = []string{"test-async-producer"}

	var (
		wg          sync.WaitGroup
		successMsgs int
		errorMsgs   int
		mu          sync.Mutex
	)

	numMessages := 5
	wg.Add(numMessages)

	producer, err := NewProducer(config, JSONEncoder{}, ProducerOptions{
		Async: true,
		ErrorHandler: func(err error) {
			mu.Lock()
			errorMsgs++
			mu.Unlock()
			wg.Done()
		},
		SuccessHandler: func(msg *sarama.ProducerMessage) {
			mu.Lock()
			successMsgs++
			mu.Unlock()
			wg.Done()
		},
	})
	require.NoError(t, err)
	defer producer.Close()

	for i := 0; i < numMessages; i++ {
		event := TestEvent{
			ID:      fmt.Sprintf("async-%d", i),
			Data:    fmt.Sprintf("test-%d", i),
			Version: 1,
		}
		err := producer.SendMessage(config.Topics[0], fmt.Sprintf("key-%d", i), event, nil)
		assert.NoError(t, err)
	}

	wg.Wait()

	assert.Equal(t, numMessages, successMsgs)
	assert.Equal(t, 0, errorMsgs)
}

func TestConsumerSingleMessages(t *testing.T) {
	config := DefaultConfig()
	config.Brokers = brokers
	config.Topics = []string{"test-single-consumer"}
	config.ConsumerGroup = "test-group-single"

	// Create producer for test messages
	producer, err := NewProducer(config, JSONEncoder{}, ProducerOptions{})
	require.NoError(t, err)
	defer producer.Close()

	// Send test messages
	testEvents := []TestEvent{
		{ID: "1", Data: "test1", Version: 1},
		{ID: "2", Data: "test2", Version: 1},
		{ID: "3", Data: "test3", Version: 1},
	}

	for _, event := range testEvents {
		err := producer.SendMessage(config.Topics[0], event.ID, event, nil)
		require.NoError(t, err)
	}

	// Create consumer with single message handler
	handler := &SingleMessageHandler{}
	consumer, err := NewConsumer[TestEvent](config, JSONEncoder{}, handler)
	require.NoError(t, err)
	defer consumer.Close()

	// Start consuming messages
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- consumer.Consume(ctx)
	}()

	// Wait for messages to be processed
	assert.Eventually(t, func() bool {
		messages := handler.GetMessages()
		return len(messages) == len(testEvents)
	}, 5*time.Second, 100*time.Millisecond)

	// Cancel the context to stop the consumer
	cancel()

	// Check for expected errors (context cancellation is okay)
	select {
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			t.Errorf("Unexpected error from Consume: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timed out waiting for consumer to exit")
	}

	// Verify received messages
	messages := handler.GetMessages()
	assert.Len(t, messages, len(testEvents))
	for i, msg := range messages {
		assert.Equal(t, testEvents[i].ID, msg.ID)
		assert.Equal(t, testEvents[i].Data, msg.Data)
	}

}

func TestConsumerBatchMessages(t *testing.T) {
	config := DefaultConfig()
	config.Brokers = brokers
	config.Topics = []string{"test-batch-consumer"}
	config.ConsumerGroup = "test-group-batch"
	// config.ConsumerSettings.BatchSize = 2
	config.ConsumerSettings.FlushInterval = 10 * time.Second

	// Create producer for test messages
	producer, err := NewProducer(config, JSONEncoder{}, ProducerOptions{})
	require.NoError(t, err)
	defer producer.Close()

	// Send test messages
	testEvents := []TestEvent{
		{ID: "1", Data: "test1", Version: 1},
		{ID: "2", Data: "test2", Version: 1},
		{ID: "3", Data: "test3", Version: 1},
	}

	for _, event := range testEvents {
		err := producer.SendMessage(config.Topics[0], event.ID, event, nil)
		require.NoError(t, err)
	}

	// Create consumer with batch handler
	handler := &BatchMessageHandler{}
	consumer, err := NewConsumer[TestEvent](config, JSONEncoder{}, handler)
	require.NoError(t, err)
	defer consumer.Close()

	// Start consuming messages
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- consumer.Consume(ctx)
	}()

	// Wait for batches to be processed
	assert.Eventually(t, func() bool {
		batches := handler.GetBatches()
		totalMessages := 0
		for _, batch := range batches {
			totalMessages += len(batch)
		}
		return totalMessages == len(testEvents)
	}, 5*time.Second, 100*time.Millisecond)

	// Cancel the context to stop the consumer
	cancel()

	// Check for expected errors
	select {
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			t.Errorf("Unexpected error from Consume: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timed out waiting for consumer to exit")
	}

	// Verify received batches
	batches := handler.GetBatches()
	totalMessages := 0
	for _, batch := range batches {
		assert.LessOrEqual(t, len(batch), config.ConsumerSettings.BatchSize)
		totalMessages += len(batch)
	}
	assert.Equal(t, len(testEvents), totalMessages)
}

func TestConsumerRetryMechanism(t *testing.T) {
	config := DefaultConfig()
	config.Brokers = brokers
	config.Topics = []string{"test-retry-consumer"}
	config.ConsumerGroup = "test-group-retry"

	// Create producer
	producer, err := NewProducer(config, JSONEncoder{}, ProducerOptions{})
	require.NoError(t, err)
	defer producer.Close()

	// Send test message
	testEvent := TestEvent{ID: "retry-1", Data: "test-retry", Version: 1}
	err = producer.SendMessage(config.Topics[0], testEvent.ID, testEvent, nil)
	require.NoError(t, err)

	// Create consumer with failing handler
	handler := &FailingHandler{maxFails: 2} // Will succeed on third attempt
	consumer, err := NewConsumer[TestEvent](config, JSONEncoder{}, handler)
	require.NoError(t, err)
	defer consumer.Close()

	// Start consuming messages
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- consumer.Consume(ctx)
	}()

	// Wait for retries to complete (3 total attempts)
	assert.Eventually(t, func() bool {
		return handler.attempts == 3
	}, 10*time.Second, 100*time.Millisecond, "Handler did not reach 3 attempts")

	// Cancel the context to trigger graceful shutdown
	cancel()

	// Check for expected errors (context.Canceled is acceptable)
	select {
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			t.Fatalf("Unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for consumer to exit")
	}

	// Verify retry attempts
	assert.Equal(t, 3, handler.attempts) // 2 failures + 1 success
}
