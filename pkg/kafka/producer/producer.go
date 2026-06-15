package producer

import (
	"context"
	"fmt"
	"sync"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/ginresponse"
	"github.com/ing-bank/golibs/pkg/kafka/errors"
	"github.com/ing-bank/golibs/pkg/kafka/stats"
	log "github.com/sirupsen/logrus"
)

var _ ProducerInterface = (*Producer)(nil)

// ProducerInterface defines the methods that a Kafka producer must implement.
type ProducerInterface interface {
	Produce(msg *kafka.Message) error
	Close() error
	MonitorEvents(ctx context.Context) chan error
	IsHealthy() bool
	Register(rg gin.IRouter)
	Stats() *stats.Statistics
	Status() *stats.Status
	DeliveryReports() <-chan kafka.Event
	Errors() <-chan error
}

// Producer represents a Kafka producer that can send messages to a Kafka topic.
type Producer struct {
	kafkaClient  *kafka.Producer
	stats        *stats.Stats
	status       *stats.Status
	monitorErrCh chan error
	wg           sync.WaitGroup
	// runtime configuration passed from Config
	dryRun     bool
	deliveryCh chan kafka.Event
	// producer options available
	isRetryableErrorFn func(error) bool
	topic              string
}

// NewForConfig creates a new Producer instance with the given configuration and options.
// It validates the provided configuration, initializes a Kafka producer, and applies
// any additional options to the Producer.
//
// Example:
//
//	config := &ProducerConfig{...}
//	producer, err := New(config)
//	if err != nil {
//	    log.Fatalf("Failed to create producer: %v", err)
//	}
func NewForConfig(cfg *Config, opts ...ProducerOption) (*Producer, error) {
	log.Printf("producer config: %+v", cfg)

	// Apply the default configuration if not provided
	ApplyDefaults(cfg)

	// Validate the configuration before creating the producer
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", errors.ErrInvalidConfig, err)
	}

	kafkaClient, err := kafka.NewProducer(cfg.ConfigMap())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errors.ErrNewClient, err)
	}

	p := &Producer{
		kafkaClient: kafkaClient,
		status:      stats.DefaultKafkaStatus(),
		dryRun:      cfg.DryRun,
		stats:       stats.New(),
		topic:       cfg.Topic,
	}

	for _, option := range opts {
		if err := option(p); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// NewMessage creates a new Kafka message with the specified value and headers.
// It uses the producer's configured topic and sets the topic partition to any available partition.
func (p *Producer) NewMessage(value []byte, headers ...kafka.Header) *kafka.Message {
	return NewMessage(value, p.topic, headers...)
}

// NewMessage creates a new Kafka message with the specified value, topic, and headers.
// It sets the topic partition to any available partition and includes any headers
func NewMessage(value []byte, topic string, headers ...kafka.Header) *kafka.Message {
	return &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          value,
		Headers:        headers,
	}
}

// Produce sends a Kafka message using the producer's Kafka client and a delivery channel.
// The message is produced to the Kafka topic specified in the message, and the delivery
// channel is used to receive delivery reports.
func (p *Producer) Produce(msg *kafka.Message) error {
	return p.kafkaClient.Produce(msg, p.deliveryCh)
}

// Close gracefully shuts down the producer, ensuring all outstanding messages are delivered.
//
// Returns:
// - An error if a fatal error occurred during the shutdown process.
func (p *Producer) Close() error {
	// Clean termination to get delivery results for all outstanding/in-transit/queued messages.
	p.kafkaClient.Flush(15_000)
	defer func() {
		p.kafkaClient.Close()
		// If a delivery channel is set, close it to signal no more messages will be sent.
		if p.deliveryCh != nil {
			close(p.deliveryCh)
		}
	}()
	// Wait for all monitoring to shut down.
	p.wg.Wait()
	// Check for fatal errors encountered by the producer.
	return p.kafkaClient.GetFatalError()
}

// Errors returns a channel that receives error messages from the Producer.
func (p *Producer) Errors() <-chan error {
	return p.monitorErrCh
}

// DeliveryReports returns a channel that receives delivery reports for messages sent by the producer.
func (p *Producer) DeliveryReports() <-chan kafka.Event {
	if p.deliveryCh == nil {
		return nil
	}
	return p.deliveryCh
}

// IsHealthy checks the health status of the producer.
func (p *Producer) IsHealthy() bool {
	return p.status.IsHealthy()
}

// Status returns the current status of the producer, including its health and statistics.
func (p *Producer) Status() *stats.Status {
	return p.status.ShowStatus()
}

// Stats returns the current Kafka statistics for the producer.
func (p *Producer) Stats() *stats.Statistics {
	return p.stats.Stats()
}

func (p *Producer) Register(rg gin.IRouter) {
	stats.RouteRegister(rg, p)
}

func (p *Producer) HandleStats(ctx *gin.Context) *ginresponse.Response {
	return p.stats.HandleStats(ctx)
}

func (p *Producer) HandleStatus(ctx *gin.Context) *ginresponse.Response {
	return p.status.HandleStatus(ctx)
}
