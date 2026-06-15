package producer

import (
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// ProducerOption allows options to be set on Producer
type ProducerOption func(p *Producer) error

// WithIsRetryableErrorFn sets the function to determine if an error is retryable.
func WithIsRetryableErrorFn(isRetryableErrorFn func(error) bool) ProducerOption {
	return func(p *Producer) error {
		p.isRetryableErrorFn = isRetryableErrorFn
		return nil
	}
}

// WithDryRun sets the dry run mode for the producer.
func WithDryRun(dryRun bool) ProducerOption {
	return func(p *Producer) error {
		p.dryRun = dryRun
		return nil
	}
}

// WithDeliveryCh sets the delivery channel for the producer.
func WithDeliveryCh(deliveryCh chan kafka.Event) ProducerOption {
	return func(p *Producer) error {
		p.deliveryCh = deliveryCh
		return nil
	}
}
