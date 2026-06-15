package producer

import (
	"context"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/ing-bank/golibs/pkg/kafka/errors"
	"github.com/ing-bank/golibs/pkg/kafka/stats"
	log "github.com/sirupsen/logrus"
)

// MonitorEvents run a go routine for serving the events channel for delivery reports and error events.
func (p *Producer) MonitorEvents(ctx context.Context) chan error {
	log.Println("Starting to monitor events")
	p.wg.Add(1)
	p.monitorErrCh = make(chan error, 1000)
	go func() {
		defer func() {
			close(p.monitorErrCh)
			p.wg.Done()
		}()
		for {
			select {
			case event := <-p.kafkaClient.Events():
				if err := p.handleEvents(event); err != nil {
					p.sendError(err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return p.monitorErrCh
}

// handleEvents processes a single Kafka event and sends errors to monitorErrCh as needed.
func (p *Producer) handleEvents(event kafka.Event) error {
	switch ev := event.(type) {
	case *kafka.Message:
		return p.handleMessageEvent(ev)
	case *kafka.Stats:
		return p.handleStatsEvent(ev)
	case kafka.Error:
		return p.handleErrorEvent(ev)
	default:
		log.Debugf("Ignored unknown Kafka event: %T", ev)
	}
	return nil
}

// handleMessageEvent checks the delivery report of a message and logs the result.
// Producing is an asynchronous operation so the client notifies the application
// of per-message produce success or failure through something called delivery reports.
// Delivery reports are by default emitted on the `.Events()` channel as `*kafka.Message`
// and you should check `msg.TopicPartition.Error` for `nil` to find out if the message
// was succesfully delivered or not.
// It is also possible to direct delivery reports to alternate channels
// by providing a non-nil `chan Event` channel to `.Produce()`.
// If no delivery reports are wanted they can be completely disabled by
// setting configuration property `"go.delivery.reports": false`.
func (p *Producer) handleMessageEvent(msg *kafka.Message) error {
	if msg.TopicPartition.Error != nil {
		return fmt.Errorf("%w: %s", errors.ErrDeliveryMsg, msg.TopicPartition.Error)
	}
	return nil
}

func (p *Producer) handleStatsEvent(ev *kafka.Stats) error {
	var kafkaStats = []byte(ev.String())
	if err := p.stats.Load(kafkaStats); err != nil {
		return fmt.Errorf("%w: failed to parse stats: %s", errors.ErrInvalidStatsData, err)
	}

	producerAssigned := p.stats.IsProducerAssigned()
	internalStatus := stats.NewInternalStatus(*p.stats.Stats(), producerAssigned)
	p.status.Refresh(internalStatus, 0)

	if !p.status.IsHealthy() {
		return fmt.Errorf("%w: producer status is down", errors.ErrProducerNotHealthy)
	}
	return nil
}

func (p *Producer) handleErrorEvent(err kafka.Error) error {
	// update the producer status based on the error
	p.status.SetCode(err.Code())
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

// sendError avoids blocking and discard further errors when the channel is full
func (p *Producer) sendError(err error) {
	select {
	case p.monitorErrCh <- err:
		// Error sent successfully
	default:
		// Channel is full, discard the error
	}
}
