// Package kafka provides functionality to monitor and manage Kafka producers and consumers.
package stats

import (
	"net/http"
	"sync"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/ginresponse"
)

type State string

func (s State) String() string {
	return string(s)
}

const (
	DownState    State = "DOWN"
	UpState      State = "UP"
	PartialState State = "PARTIAL"
	UnknownState State = "UNKNOWN"
)

type Status struct {
	*InternalStatus
	mu *sync.RWMutex
}

// InternalStatus represents the status of Kafka components including the overall status, error code, and details.
type InternalStatus struct {
	Status  string          `json:"status"`
	Code    kafka.ErrorCode `json:"code"`
	Details Details         `json:"details"`
}

// Details contains information about Kafka consuming status.
type Details struct {
	KafkaConsuming KafkaConsuming `json:"kafkaConsuming"`
}

// KafkaConsuming represents the status of Kafka consuming.
type KafkaConsuming struct {
	Status string `json:"status"`
}

// DefaultKafkaStatus returns a default Status instance with predefined values.
func DefaultKafkaStatus() *Status {
	return &Status{
		InternalStatus: &InternalStatus{
			Status: DownState.String(), // Default to down until proven otherwise
			Details: Details{
				KafkaConsuming: KafkaConsuming{Status: DownState.String()},
			},
			Code: kafka.ErrNoError,
		},
		mu: &sync.RWMutex{},
	}
}

// SetCode sets the error code for the status.
func (s *Status) SetCode(code kafka.ErrorCode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Code = code
}

// ShowStatus returns the current status.
func (s *Status) ShowStatus() *Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s
}

// IsHealthy checks if the overall status and Kafka consuming status are healthy.
func (s *Status) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Status == DownState.String() {
		return false
	}
	if s.Details.KafkaConsuming.Status == DownState.String() {
		return false
	}
	return s.Code == 0
}

// NewInternalStatus creates a new Status instance based on the current Statistics and consuming state.
func NewInternalStatus(s Statistics, consuming bool) *InternalStatus {
	// Determine the overall broker status
	brokerStatus := ParseStatus(s.Brokers)
	stateup := DownState
	if consuming {
		stateup = UpState
	}
	return &InternalStatus{
		Status: brokerStatus.String(),
		Code:   0,
		Details: Details{
			KafkaConsuming: KafkaConsuming{
				Status: stateup.String(),
			},
		},
	}
}

// Refresh updates the current status with a new status and error code.
func (s *Status) Refresh(status *InternalStatus, code kafka.ErrorCode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status != nil {
		s.Status = status.Status
		s.Details = status.Details
	}
	s.Code = code
}

// SetDown sets the overall status and Kafka consuming status to down.
func (s *Status) SetDown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = DownState.String()
	s.Details.KafkaConsuming.Status = DownState.String()
}

// ParseStatus parses the status of Kafka brokers and determines the overall state.
func ParseStatus(brokers map[string]map[string]any) State {
	if len(brokers) == 0 {
		return UnknownState
	}
	upCount := 0
	downCount := 0
	for _, m := range brokers {
		if m["state"] == UpState.String() {
			upCount++
		} else if m["state"] == DownState.String() {
			downCount++
		}
	}
	switch {
	case upCount == len(brokers):
		return UpState
	case downCount == len(brokers):
		return DownState
	case upCount == 0 && downCount == 0:
		return UnknownState
	default:
		return PartialState
	}
}

func (c *Status) HandleStatus(_ *gin.Context) *ginresponse.Response {
	status := c.ShowStatus()
	if !c.IsHealthy() {
		return ginresponse.New(status).WithStatus(http.StatusServiceUnavailable)
	}
	return ginresponse.New(status).WithStatus(http.StatusOK)
}
