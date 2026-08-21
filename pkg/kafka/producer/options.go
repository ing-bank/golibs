package producer

import (
	"time"

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

// ProduceOption sets options on the Produce method. Note, for initialization of the producer, use ProducerOption instead.
type ProduceOption func(p *ProduceOptions) error

// ProduceOptions are options on the Produce method. Note, for initialization of the producer, use ProducerOption instead.
type ProduceOptions struct {
	WaitForDeliveryTimeout time.Duration
	WaitForDelivery        bool
}

func NewProduceOptions() *ProduceOptions {
	opts := &ProduceOptions{}
	opts.ApplyDefaults()
	return opts
}

func (p *ProduceOptions) ApplyDefaults() {
	if p.WaitForDeliveryTimeout == 0 {
		p.WaitForDeliveryTimeout = time.Second * 10
	}
}

// WithWaitForDelivery sets the WaitForDelivery option for the Produce method. If set to true, the producer will wait
// for delivery reports before returning from the Produce method.
func WithWaitForDelivery(wait bool) ProduceOption {
	return func(p *ProduceOptions) error {
		p.WaitForDelivery = wait
		return nil
	}
}

// WithWaitForDeliveryTimeout sets the timeout used when waiting for delivery reports.
func WithWaitForDeliveryTimeout(timeout time.Duration) ProduceOption {
	return func(p *ProduceOptions) error {
		p.WaitForDeliveryTimeout = timeout
		return nil
	}
}

