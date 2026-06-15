package errors

import (
	"errors"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

var (
	ErrInvalidConfig      = fmt.Errorf("invalid config")
	ErrNewClient          = fmt.Errorf("failed to instantiate client")
	ErrSubscribe          = fmt.Errorf("failed to subscribe to topic")
	ErrNoContent          = fmt.Errorf("no content")
	ErrInvalidStatsData   = fmt.Errorf("invalid stats json format data")
	ErrOffsetNotStored    = fmt.Errorf("offset not stored")
	ErrCommitFailed       = fmt.Errorf("failed to commit offsets")
	ErrProducerNotHealthy = fmt.Errorf("producer not healthy")
	ErrConsumerNotHealthy = fmt.Errorf("consumer not healthy")
	ErrDeliveryMsg        = fmt.Errorf("failed to deliver message")
	ErrMsgIgnored         = fmt.Errorf("ignored message")
	ErrNotRetriable       = fmt.Errorf("operation is not retriable")
	ErrFatalError         = errors.New(kafka.ErrFatal.String())
	ErrAllBrokersDown     = errors.New(kafka.ErrAllBrokersDown.String())
)

// IsRetryableErrorFn determines if an error is retryable.
func IsRetryableErrorFn(err error) bool {
	if kafkaerr, ok := err.(*kafka.Error); ok {
		if kafkaerr.IsRetriable() || kafkaerr.IsTimeout() {
			return true
		}
	}
	return false
}
