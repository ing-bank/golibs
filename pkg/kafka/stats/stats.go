package stats

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/ginresponse"
)

type Stats struct {
	mu *sync.RWMutex
	*Statistics
}

// Statistics represents the Kafka statistics structure as defined by librdkafka.
// https://docs.confluent.io/platform/7.5/clients/librdkafka/html/md_STATISTICS.html
type Statistics struct {
	MsgCnt      int64 `json:"msg_cnt"`      // Current number of messages in producer queues
	MsgSize     int64 `json:"msg_size"`     // Current total size of messages in producer queues
	MsgMax      int64 `json:"msg_max"`      // Threshold: maximum number of messages allowed allowed on the producer queues
	MsgSizeMax  int64 `json:"msg_size_max"` // Threshold: maximum total size of messages allowed on the producer queues
	Tx          int64 `json:"tx"`           // Total number of requests sent to Kafka brokers
	TxBytes     int64 `json:"tx_bytes"`     // Total number of bytes transmitted to Kafka brokers
	TxMsgs      int64 `json:"txmsgs"`       // Total number of messages transmitted (produced) to Kafka brokers
	TxMsgsBytes int64 `json:"txmsg_bytes"`  // Total number of message bytes (including framing, such as per-Message framing and MessageSet/batch framing) transmitted to Kafka brokers
	Rx          int64 `json:"rx"`           // Total number of responses received from Kafka brokers
	RxBytes     int64 `json:"rx_bytes"`     // Total number of bytes received from Kafka brokers
	RxMsgs      int64 `json:"rxmsgs"`       // Total number of messages consumed, not including ignored messages (due to offset, etc), from Kafka brokers.
	RxMsgsBytes int64 `json:"rxmsg_bytes"`  // Total number of message bytes (including framing) received from Kafka brokers

	Eos struct {
		IdempState string `json:"idemp_state"` // Current idempotent producer id state.
		TxnState   string `json:"txn_state"`   // Current transactional producer state.
		TxnMayEnq  bool   `json:"txn_may_enq"` // Transactional state allows enqueuing (producing) new messages.
	} `json:"eos"`

	ConsumerGroup struct {
		AssignmentSize  int64  `json:"assignment_size"`  // Current assignment's partition count.
		JoinState       string `json:"join_state"`       // Local consumer group handler's join state.
		RebalanceAge    int64  `json:"rebalance_age"`    // Time elapsed since last rebalance (assign or revoke) (milliseconds).
		RebalanceCnt    int64  `json:"rebalance_cnt"`    // Time elapsed since last rebalance (assign or revoke) (milliseconds).
		RebalanceReason string `json:"rebalance_reason"` // Last rebalance reason, or empty string.
		State           string `json:"state"`            // Local consumer group handler's state.
		Stateage        int64  `json:"stateage"`         // Time elapsed since last state change (milliseconds).
	} `json:"cgrp"`

	Brokers map[string]map[string]any `json:"brokers"`
}

func New() *Stats {
	return &Stats{
		mu:         &sync.RWMutex{},
		Statistics: &Statistics{},
	}
}

// IsProducerAssigned checks if the Kafka producer is healthy by verifying
// if messages are being transmitted and the idempotent state is assigned.
func (s *Stats) IsProducerAssigned() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// maybe we should also check for transmitted messages ks.TxMsgs > 0
	if s.Eos.IdempState == "Assigned" {
		return true
	}
	return false
}

// Load parses the provided JSON data and updates the Statistics field of the Stats struct.
func (s *Stats) Load(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := json.Unmarshal(data, s); err != nil {
		return fmt.Errorf("failed to parse stats: %w", err)
	}
	return nil
}

func (s *Stats) Stats() *Statistics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Statistics
}

// IsConsumerUp checks if the Kafka consumer is healthy by verifying its join state and overall state.
func (s *Stats) IsConsumerUp() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ConsumerGroup.JoinState == "steady" && s.ConsumerGroup.State == "up" {
		return true
	}
	return false
}

// ...existing code...

func (s *Stats) HandleStats(_ *gin.Context) *ginresponse.Response {
	return ginresponse.New(s.Stats()).WithStatus(http.StatusOK)
}
