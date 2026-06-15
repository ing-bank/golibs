package consumer

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

type ConsumerInterface interface {
	HandleEvents(ctx context.Context, processMessageFunc ProcessMessageFunc) <-chan ProcessingError
	IsClosed() bool
	Close() error
	IsHealthy() bool
	Wait()
	Register(rg gin.IRouter)
	Stats() *stats.Statistics
	Status() *stats.Status
	Errors() <-chan ProcessingError
	StoreMessage(msg *kafka.Message) error
	Commit() error
	CommitMessage(msg *kafka.Message) error
}

var _ ConsumerInterface = (*Consumer)(nil)

type ProcessMessageFunc func(ctx context.Context, msg *kafka.Message) error

type Consumer struct {
	kafkaClient      *kafka.Consumer
	wg               sync.WaitGroup
	stats            *stats.Stats
	status           *stats.Status
	enableAutoCommit bool
	errCh            chan ProcessingError
}

type ProcessingError struct {
	Event kafka.Event
	Err   error
}

func NewForConfig(c *Config) (*Consumer, error) {
	log.Printf("consumer config: %+v", c)

	cfg := *c // shallow copy

	// Apply the default configuration if not provided
	ApplyDefaults(&cfg)

	// Validate the configuration before creating the consumer
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", errors.ErrInvalidConfig, err)
	}

	// https://github.com/confluentinc/librdkafka/blob/master/CONFIGURATION.md
	consumer, err := kafka.NewConsumer(cfg.ConfigMap())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errors.ErrNewClient, err)
	}

	err = consumer.SubscribeTopics(cfg.Topics, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errors.ErrSubscribe, err)
	}

	return &Consumer{
		kafkaClient:      consumer,
		enableAutoCommit: !cfg.DryRun,
		status:           stats.DefaultKafkaStatus(),
		errCh:            make(chan ProcessingError, 1000),
		stats:            stats.New(),
	}, nil
}

// HandleEvents processes events inside a goroutine using the provided processMessageFunc.
// This function runs asynchronously, call Wait() to hold until the context is canceled.
func (c *Consumer) HandleEvents(ctx context.Context, processMessageFunc ProcessMessageFunc) <-chan ProcessingError {
	c.wg.Add(1)
	go func() {
		defer func() {
			close(c.errCh)
			c.wg.Done()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			ev := c.kafkaClient.Poll(5_000)
			if ev == nil {
				continue
			}
			if err := c.handleEvent(ctx, processMessageFunc, ev); err != nil {
				c.sendError(ev, err)
			}
		}
	}()
	return c.errCh
}

// handleEvent processes the event and calls the appropriate handler based on its type.
func (c *Consumer) handleEvent(ctx context.Context, processMessageFunc ProcessMessageFunc, event kafka.Event) error {
	switch ev := event.(type) {
	case *kafka.Message:
		return c.handleMessageEvent(ctx, processMessageFunc, ev)
	case *kafka.Stats:
		return c.handleStatsEvent(ev)
	case kafka.Error:
		return c.handleErrorEvent(ev)
	case kafka.OffsetsCommitted:
		return nil
	default:
		return fmt.Errorf("%w: %s", errors.ErrMsgIgnored, ev.String())
	}
}

func (c *Consumer) handleMessageEvent(ctx context.Context, processMessageFunc ProcessMessageFunc, msg *kafka.Message) error {
	if msg == nil || len(msg.Value) == 0 {
		return fmt.Errorf("%w: %v", errors.ErrNoContent, msg)
	}
	if processMessageFunc == nil {
		return fmt.Errorf("error: no processMessageFunc provided")
	}
	if err := processMessageFunc(ctx, msg); err != nil {
		return err
	}
	if !c.enableAutoCommit {
		return nil
	}
	return c.StoreMessage(msg)
}

// handleStatsEvent processes the stats event and updates the consumer status.
// https://github.com/confluentinc/librdkafka/blob/master/STATISTICS.md
func (c *Consumer) handleStatsEvent(ev *kafka.Stats) error {
	var kafkaStats = []byte(ev.String())
	if err := c.stats.Load(kafkaStats); err != nil {
		return fmt.Errorf("%w: '%s': %w", errors.ErrInvalidStatsData, ev, err)
	}

	isConsumerUp := c.stats.IsConsumerUp()
	internalStatus := stats.NewInternalStatus(*c.stats.Stats(), isConsumerUp)
	c.status.Refresh(internalStatus, 0)

	if !c.status.IsHealthy() {
		return fmt.Errorf("%w: consumer status is down", errors.ErrConsumerNotHealthy)
	}
	return nil
}

// handleErrorEvent processes the error event and updates the consumer status.
// https://pkg.go.dev/github.com/confluentinc/confluent-kafka-go/v2/kafka@v2.3.0#ErrBadMsg
func (c *Consumer) handleErrorEvent(err kafka.Error) error {
	// update the producer status based on the error
	c.status.SetCode(err.Code())
	// https://pkg.go.dev/github.com/confluentinc/confluent-kafka-go/v2/kafka@v2.3.0#ErrBadMsg
	if err.IsFatal() {
		return errors.ErrFatalError
	}
	if !err.IsRetriable() || err.TxnRequiresAbort() {
		return errors.ErrNotRetriable
	}
	if err.Code() == kafka.ErrAllBrokersDown {
		return errors.ErrAllBrokersDown
	}
	return err
}

// StoreMessage stores the message offset if auto-commit is enabled.
func (c *Consumer) StoreMessage(msg *kafka.Message) error {
	if !c.enableAutoCommit {
		return nil
	}
	if _, err := c.kafkaClient.StoreMessage(msg); err != nil {
		return fmt.Errorf("%w: %s", errors.ErrOffsetNotStored, msg.TopicPartition)
	}
	return nil
}

// Commit commits the offsets of the messages that have been processed.
func (c *Consumer) Commit() error {
	if _, err := c.kafkaClient.Commit(); err != nil {
		return fmt.Errorf("%w: %s", errors.ErrCommitFailed, err)
	}
	return nil
}

// CommitMessage commits a specific message offset even if auto-commit is disabled or dry-run is enabled.
func (c *Consumer) CommitMessage(msg *kafka.Message) error {
	if msg == nil {
		return fmt.Errorf("%w: message is nil", errors.ErrNoContent)
	}
	if _, err := c.kafkaClient.CommitMessage(msg); err != nil {
		return fmt.Errorf("%w: %s", errors.ErrCommitFailed, err)
	}
	return nil
}

// Close Consumer instance. The object is no longer usable after this call.
func (c *Consumer) Close() error {
	return c.kafkaClient.Close()
}

// IsClosed checks if the consumer is closed.
func (c *Consumer) IsClosed() bool {
	return c.kafkaClient.IsClosed()
}

// IsHealthy checks if the consumer is healthy by verifying the status of all brokers and the consumer itself.
func (c *Consumer) IsHealthy() bool {
	return c.status.IsHealthy()
}

// sendError avoids blocking and discard further errors when the channel is full
func (c *Consumer) sendError(ev kafka.Event, err error) {
	select {
	case c.errCh <- ProcessingError{Event: ev, Err: err}:
		// Error sent successfully
	default:
		// Channel is full, discard the error
	}
}

// Errors returns a channel that receives error messages from the Consumer.
func (c *Consumer) Errors() <-chan ProcessingError {
	return c.errCh
}

// Status returns the current status of the consumer, including its health and statistics.
func (c *Consumer) Status() *stats.Status {
	return c.status.ShowStatus()
}

// Stats returns the current Kafka statistics for the consumer.
func (c *Consumer) Stats() *stats.Statistics {
	return c.stats.Stats()
}

// Wait blocks until all goroutines launched by HandleEvents have completed.
// This function should be called to ensure all events are processed before exiting.
func (c *Consumer) Wait() {
	c.wg.Wait()
}

func (c *Consumer) Register(rg gin.IRouter) {
	stats.RouteRegister(rg, c)
}

func (c *Consumer) HandleStats(ctx *gin.Context) *ginresponse.Response {
	return c.stats.HandleStats(ctx)
}

func (c *Consumer) HandleStatus(ctx *gin.Context) *ginresponse.Response {
	return c.status.HandleStatus(ctx)
}
