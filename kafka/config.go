package kafka

import (
	"errors"
	"time"

	"github.com/IBM/sarama"
)

type KafkaConfig struct {
	Brokers          []string
	Topics           []string
	ConsumerGroup    string
	ProducerSettings ProducerConfig
	ConsumerSettings ConsumerConfig
	NetworkSettings  NetworkConfig
	SecuritySettings SecurityConfig
}

type ProducerConfig struct {
	RequiredAcks     sarama.RequiredAcks
	RetryMax         int
	FlushFrequency   time.Duration
	FlushMaxMessages int
	CompressionType  sarama.CompressionCodec
}

type ConsumerConfig struct {
	BalanceStrategy    []sarama.BalanceStrategy
	InitialOffset      int64
	SessionTimeout     time.Duration
	HeartbeatInterval  time.Duration
	MaxProcessingTime  time.Duration
	AutoCommitInterval time.Duration
	BatchSize          int
	FlushInterval      time.Duration
}

type NetworkConfig struct {
	MaxOpenRequests int
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
}

type SecurityConfig struct {
	EnableTLS     bool
	EnableSASL    bool
	SASLMechanism string
	SASLUsername  string
	SASLPassword  string
}

func DefaultConfig() *KafkaConfig {
	return &KafkaConfig{
		ProducerSettings: ProducerConfig{
			RequiredAcks:     sarama.WaitForLocal,
			RetryMax:         3,
			FlushFrequency:   500 * time.Millisecond,
			FlushMaxMessages: 1000,
			CompressionType:  sarama.CompressionSnappy,
		},
		ConsumerSettings: ConsumerConfig{
			BalanceStrategy:    []sarama.BalanceStrategy{sarama.NewBalanceStrategyRange()},
			InitialOffset:      sarama.OffsetOldest,
			SessionTimeout:     30 * time.Second,
			HeartbeatInterval:  3 * time.Second,
			MaxProcessingTime:  300 * time.Second,
			AutoCommitInterval: 1 * time.Second,
		},
		NetworkSettings: NetworkConfig{
			MaxOpenRequests: 5,
			DialTimeout:     30 * time.Second,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
		},
	}
}

func (c *KafkaConfig) Validate() error {
	if len(c.Brokers) == 0 {
		return errors.New("at least one broker must be configured")
	}
	if c.SecuritySettings.EnableSASL {
		if c.SecuritySettings.SASLUsername == "" || c.SecuritySettings.SASLPassword == "" {
			return errors.New("SASL requires both username and password")
		}
	}
	return nil
}

func (c *KafkaConfig) SaramaConfig() (*sarama.Config, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	config := sarama.NewConfig()

	// Producer Configuration
	config.Producer.RequiredAcks = c.ProducerSettings.RequiredAcks
	config.Producer.Retry.Max = c.ProducerSettings.RetryMax
	config.Producer.Flush.Frequency = c.ProducerSettings.FlushFrequency
	config.Producer.Flush.MaxMessages = c.ProducerSettings.FlushMaxMessages
	config.Producer.Compression = c.ProducerSettings.CompressionType
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true

	// Consumer Configuration
	config.Consumer.Group.Rebalance.GroupStrategies = c.ConsumerSettings.BalanceStrategy
	config.Consumer.Offsets.Initial = c.ConsumerSettings.InitialOffset
	config.Consumer.Group.Session.Timeout = c.ConsumerSettings.SessionTimeout
	config.Consumer.Group.Heartbeat.Interval = c.ConsumerSettings.HeartbeatInterval
	config.Consumer.MaxProcessingTime = c.ConsumerSettings.MaxProcessingTime
	config.Consumer.Offsets.AutoCommit.Interval = c.ConsumerSettings.AutoCommitInterval
	config.Consumer.Return.Errors = true

	// Network Configuration
	config.Net.MaxOpenRequests = c.NetworkSettings.MaxOpenRequests
	config.Net.DialTimeout = c.NetworkSettings.DialTimeout
	config.Net.ReadTimeout = c.NetworkSettings.ReadTimeout
	config.Net.WriteTimeout = c.NetworkSettings.WriteTimeout

	// Security Configuration
	if c.SecuritySettings.EnableTLS {
		config.Net.TLS.Enable = true
	}
	if c.SecuritySettings.EnableSASL {
		config.Net.SASL.Enable = true
		config.Net.SASL.Mechanism = sarama.SASLMechanism(c.SecuritySettings.SASLMechanism)
		config.Net.SASL.User = c.SecuritySettings.SASLUsername
		config.Net.SASL.Password = c.SecuritySettings.SASLPassword
	}

	return config, nil
}
