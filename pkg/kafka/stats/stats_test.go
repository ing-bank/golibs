package stats

import (
	"reflect"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()
	t.Run("new stats is not nil", func(t *testing.T) {
		t.Parallel()
		got := New()
		if got == nil || got.Statistics == nil || got.mu == nil {
			t.Errorf("New() = %v, want non-nil fields", got)
		}
	})
}

func TestStats_Load(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name:    "valid json",
			input:   []byte(`{"msg_cnt":1}`),
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   []byte(`{"msg_cnt":}`),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mu := &sync.RWMutex{}
			ks := &Stats{mu: mu, Statistics: &Statistics{}}
			err := ks.Load(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(ks.mu, ks.mu) {
				t.Errorf("Load() changed mutex reference")
			}
		})
	}
}

func TestStats_Load_LargePayload(t *testing.T) {
	t.Parallel()
	payload := []byte(`
{
    "msg_cnt": 0,
    "msg_size": 0,
    "msg_max": 0,
    "msg_size_max": 0,
    "tx": 1763,
    "tx_bytes": 170179,
    "txmsgs": 0,
    "txmsg_bytes": 0,
    "rx": 1762,
    "rx_bytes": 128794,
    "rxmsgs": 0,
    "rxmsg_bytes": 0,
    "eos": {
        "idemp_state": "",
        "txn_state": "",
        "txn_may_enq": false
    },
    "cgrp": {
        "assignment_size": 1,
        "join_state": "steady",
        "rebalance_age": 917451,
        "rebalance_cnt": 1,
        "rebalance_reason": "",
        "state": "up",
        "stateage": 920622
    },
    "brokers": {
        "GroupCoordinator": {
            "state": "UP"
        },
        "br101-odin-dev.ic.ing.net:9092/101": {
            "state": "INIT"
        },
        "br201-odin-dev.ic.ing.net:9092/201": {
            "state": "INIT"
        },
        "br301-odin-dev.ic.ing.net:9092/301": {
            "state": "INIT"
        },
        "br401-odin-dev.ic.ing.net:9092/401": {
            "state": "UP"
        }
    }
}
`)
	ks := &Stats{mu: &sync.RWMutex{}, Statistics: &Statistics{}}
	err := ks.Load(payload)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if ks.Tx != 1763 || ks.Rx != 1762 {
		t.Errorf("Tx or Rx not loaded correctly: Tx=%v, Rx=%v", ks.Tx, ks.Rx)
	}
	if ks.ConsumerGroup.JoinState != "steady" || ks.ConsumerGroup.State != "up" {
		t.Errorf("ConsumerGroup fields not loaded correctly: %+v", ks.ConsumerGroup)
	}
	if len(ks.Brokers) != 5 {
		t.Errorf("Expected 5 brokers, got %d", len(ks.Brokers))
	}
	if ks.Brokers["GroupCoordinator"]["state"] != "UP" {
		t.Errorf("Expected GroupCoordinator state UP, got %v", ks.Brokers["GroupCoordinator"]["state"])
	}
}

func TestStats_Stats(t *testing.T) {
	t.Parallel()
	ks := &Stats{mu: &sync.RWMutex{}, Statistics: &Statistics{MsgCnt: 42}}
	if got := ks.Stats(); got.MsgCnt != 42 {
		t.Errorf("Stats() = %v, want MsgCnt=42", got)
	}
}

func TestStats_IsProducerAssigned(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		txMsgs int64
		idemp  string
		want   bool
	}{
		{
			name:   "healthy producer",
			txMsgs: 10,
			idemp:  "Assigned",
			want:   true,
		},
		{
			name:   "unhealthy producer",
			txMsgs: 0,
			idemp:  "Unassigned",
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ks := &Stats{mu: &sync.RWMutex{}, Statistics: &Statistics{TxMsgs: tt.txMsgs}}
			ks.Eos.IdempState = tt.idemp
			got := ks.IsProducerAssigned()
			if got != tt.want {
				t.Errorf("IsProducerAssigned() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKafkaStats_IsConsumerHealthy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		group struct {
			JoinState string
			State     State
		}
		want bool
	}{
		{
			name: "healthy consumer",
			group: struct {
				JoinState string
				State     State
			}{"steady", "up"},
			want: true,
		},
		{
			name: "unhealthy consumer - wrong state",
			group: struct {
				JoinState string
				State     State
			}{"joining", "down"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ks := &Stats{
				mu: &sync.RWMutex{},
				Statistics: &Statistics{
					ConsumerGroup: struct {
						AssignmentSize  int64  `json:"assignment_size"`
						JoinState       string `json:"join_state"`
						RebalanceAge    int64  `json:"rebalance_age"`
						RebalanceCnt    int64  `json:"rebalance_cnt"`
						RebalanceReason string `json:"rebalance_reason"`
						State           string `json:"state"`
						Stateage        int64  `json:"stateage"`
					}{
						JoinState: tt.group.JoinState,
						State:     tt.group.State.String(),
					},
				},
			}
			got := ks.IsConsumerUp()
			if got != tt.want {
				t.Errorf("IsConsumerUp() got = %v, want %v", got, tt.want)
			}
		})
	}
}
