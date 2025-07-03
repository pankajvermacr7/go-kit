package kafka

import (
	"github.com/IBM/sarama"
)

type Producer struct {
	syncProducer  sarama.SyncProducer
	asyncProducer sarama.AsyncProducer
	encoder       Encoder
	metrics       *ProducerMetrics
}

type ProducerOptions struct {
	Async          bool
	ErrorHandler   func(error)
	SuccessHandler func(*sarama.ProducerMessage)
}

func NewProducer(config *KafkaConfig, encoder Encoder, opts ProducerOptions) (*Producer, error) {
	saramaConfig, err := config.SaramaConfig()
	if err != nil {
		return nil, err
	}

	var (
		syncProd  sarama.SyncProducer
		asyncProd sarama.AsyncProducer
	)

	if opts.Async {
		asyncProd, err = sarama.NewAsyncProducer(config.Brokers, saramaConfig)
	} else {
		syncProd, err = sarama.NewSyncProducer(config.Brokers, saramaConfig)
	}

	if err != nil {
		return nil, err
	}

	p := &Producer{
		syncProducer:  syncProd,
		asyncProducer: asyncProd,
		encoder:       encoder,
		metrics:       NewProducerMetrics(),
	}

	if opts.Async {
		go p.handleAsyncResults(opts.ErrorHandler, opts.SuccessHandler)
	}

	return p, nil
}

func (p *Producer) SendMessage(topic string, key string, value interface{}, headers map[string]string) error {
	msgBytes, err := p.encoder.Encode(value)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(msgBytes),
	}

	for k, v := range headers {
		msg.Headers = append(msg.Headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}

	if p.asyncProducer != nil {
		p.asyncProducer.Input() <- msg
		p.metrics.MessagesProduced.WithLabelValues(topic).Inc()
		return nil
	}

	_, _, err = p.syncProducer.SendMessage(msg)
	if err == nil {
		p.metrics.MessagesProduced.WithLabelValues(topic).Inc()
	}
	return err
}

func (p *Producer) handleAsyncResults(errHandler func(error), successHandler func(*sarama.ProducerMessage)) {
	successes := p.asyncProducer.Successes()
	errors := p.asyncProducer.Errors()

	go func() {
		// Check if both channels are closed
		for {
			if successes == nil && errors == nil {
				return
			}
			select {
			case err, ok := <-p.asyncProducer.Errors():
				if !ok {
					errors = nil
					continue
				}
				if errHandler != nil {
					errHandler(err)
				}
			case msg, ok := <-p.asyncProducer.Successes():
				if !ok {
					successes = nil
					continue
				}
				if successHandler != nil {
					successHandler(msg)
				}
			}
		}
	}()

}

func (p *Producer) Close() error {
	if p.asyncProducer != nil {
		return p.asyncProducer.Close()
	}
	return p.syncProducer.Close()
}
