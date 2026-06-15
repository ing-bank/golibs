package stats

import (
	"sync"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func TestDefaultKafkaStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want *Status
	}{
		{
			name: "default status",
			want: &Status{
				InternalStatus: &InternalStatus{
					Status: DownState.String(),
					Details: Details{
						KafkaConsuming: KafkaConsuming{Status: DownState.String()},
					},
					Code: kafka.ErrNoError,
				},
				mu: &sync.RWMutex{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DefaultKafkaStatus()
			if got.Status != tt.want.Status || got.Code != tt.want.Code || got.Details.KafkaConsuming.Status != tt.want.Details.KafkaConsuming.Status {
				t.Errorf("DefaultKafkaStatus() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestStats_NewStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		brokers       map[string]map[string]any
		consuming     bool
		want          string
		wantConsuming string
	}{
		{
			name:          "all up, consuming",
			brokers:       map[string]map[string]any{"b1": {"state": UpState.String()}, "b2": {"state": UpState.String()}},
			consuming:     true,
			want:          UpState.String(),
			wantConsuming: UpState.String(),
		},
		{
			name:          "all down, not consuming",
			brokers:       map[string]map[string]any{"b1": {"state": DownState.String()}, "b2": {"state": DownState.String()}},
			consuming:     false,
			want:          DownState.String(),
			wantConsuming: DownState.String(),
		},
		{
			name:          "partial, consuming",
			brokers:       map[string]map[string]any{"b1": {"state": UpState.String()}, "b2": {"state": DownState.String()}},
			consuming:     true,
			want:          PartialState.String(),
			wantConsuming: UpState.String(),
		},
		{
			name:          "unknown, not consuming",
			brokers:       map[string]map[string]any{},
			consuming:     false,
			want:          UnknownState.String(),
			wantConsuming: DownState.String(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			internalStatus := NewInternalStatus(Statistics{Brokers: tt.brokers}, tt.consuming)
			status := &Status{InternalStatus: internalStatus, mu: &sync.RWMutex{}}
			if status.Status != tt.want || status.Details.KafkaConsuming.Status != tt.wantConsuming {
				t.Errorf("NewInternalStatus() = %v/%v, want %v/%v", status.Status, status.Details.KafkaConsuming.Status, tt.want, tt.wantConsuming)
			}
		})
	}
}

func TestStatus_ShowStatus(t *testing.T) {
	t.Parallel()
	s := &Status{
		InternalStatus: &InternalStatus{
			Status: "foo",
		},
		mu: &sync.RWMutex{},
	}
	got := s.ShowStatus()
	if got.Status != "foo" {
		t.Errorf("ShowStatus() = %v, want foo", got.Status)
	}
}

func TestStatus_SetCode(t *testing.T) {
	t.Parallel()
	s := &Status{InternalStatus: &InternalStatus{}, mu: &sync.RWMutex{}}
	s.SetCode(kafka.ErrAllBrokersDown)
	if s.Code != kafka.ErrAllBrokersDown {
		t.Errorf("SetCode() = %v, want %v", s.Code, kafka.ErrAllBrokersDown)
	}
}

func TestStatus_IsHealthy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		state     string
		consuming string
		code      kafka.ErrorCode
		want      bool
	}{
		{"all up", UpState.String(), UpState.String(), kafka.ErrNoError, true},
		{"down", DownState.String(), DownState.String(), kafka.ErrNoError, false},
		{"consuming down", UpState.String(), DownState.String(), kafka.ErrNoError, false},
		{"error code", UpState.String(), UpState.String(), kafka.ErrAllBrokersDown, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &Status{
				InternalStatus: &InternalStatus{
					Status:  tt.state,
					Details: Details{KafkaConsuming: KafkaConsuming{Status: tt.consuming}},
					Code:    tt.code,
				},
				mu: &sync.RWMutex{},
			}
			if got := s.IsHealthy(); got != tt.want {
				t.Errorf("IsHealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		brokers map[string]map[string]any
		want    State
	}{
		{
			name:    "all up",
			brokers: map[string]map[string]any{"b1": {"state": UpState.String()}, "b2": {"state": UpState.String()}},
			want:    UpState,
		},
		{
			name:    "all down",
			brokers: map[string]map[string]any{"b1": {"state": DownState.String()}, "b2": {"state": DownState.String()}},
			want:    DownState,
		},
		{
			name:    "partial",
			brokers: map[string]map[string]any{"b1": {"state": UpState.String()}, "b2": {"state": DownState.String()}},
			want:    PartialState,
		},
		{
			name:    "unknown",
			brokers: map[string]map[string]any{"b1": {"state": "INIT"}},
			want:    UnknownState,
		},
		{
			name:    "empty",
			brokers: map[string]map[string]any{},
			want:    UnknownState,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ParseStatus(tt.brokers)
			if got != tt.want {
				t.Errorf("ParseStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_SetDown(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		start         *Status
		want          string
		wantConsuming string
	}{
		{
			name:          "set down from up",
			start:         &Status{InternalStatus: &InternalStatus{Status: UpState.String(), Details: Details{KafkaConsuming: KafkaConsuming{Status: UpState.String()}}}, mu: &sync.RWMutex{}},
			want:          DownState.String(),
			wantConsuming: DownState.String(),
		},
		{
			name:          "set down from partial",
			start:         &Status{InternalStatus: &InternalStatus{Status: PartialState.String(), Details: Details{KafkaConsuming: KafkaConsuming{Status: PartialState.String()}}}, mu: &sync.RWMutex{}},
			want:          DownState.String(),
			wantConsuming: DownState.String(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.start.SetDown()
			if tt.start.Status != tt.want || tt.start.Details.KafkaConsuming.Status != tt.wantConsuming {
				t.Errorf("SetDown() = %v/%v, want %v/%v", tt.start.Status, tt.start.Details.KafkaConsuming.Status, tt.want, tt.wantConsuming)
			}
		})
	}
}

func TestStatus_Refresh(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		start         *Status
		update        *Status
		code          kafka.ErrorCode
		want          string
		wantConsuming string
		wantCode      kafka.ErrorCode
	}{
		{
			name:          "refresh with new status",
			start:         &Status{InternalStatus: &InternalStatus{Status: DownState.String(), Details: Details{KafkaConsuming: KafkaConsuming{Status: DownState.String()}}}, mu: &sync.RWMutex{}},
			update:        &Status{InternalStatus: &InternalStatus{Status: UpState.String(), Details: Details{KafkaConsuming: KafkaConsuming{Status: UpState.String()}}}, mu: &sync.RWMutex{}},
			code:          kafka.ErrNoError,
			want:          UpState.String(),
			wantConsuming: UpState.String(),
			wantCode:      kafka.ErrNoError,
		},
		{
			name:          "refresh with nil status",
			start:         &Status{InternalStatus: &InternalStatus{Status: PartialState.String(), Details: Details{KafkaConsuming: KafkaConsuming{Status: PartialState.String()}}}, mu: &sync.RWMutex{}},
			update:        nil,
			code:          kafka.ErrAllBrokersDown,
			want:          PartialState.String(),
			wantConsuming: PartialState.String(),
			wantCode:      kafka.ErrAllBrokersDown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var updateInternal *InternalStatus
			if tt.update != nil {
				updateInternal = tt.update.InternalStatus
			}
			tt.start.Refresh(updateInternal, tt.code)
			if tt.start.Status != tt.want || tt.start.Details.KafkaConsuming.Status != tt.wantConsuming || tt.start.Code != tt.wantCode {
				t.Errorf("Refresh() = %v/%v/%v, want %v/%v/%v", tt.start.Status, tt.start.Details.KafkaConsuming.Status, tt.start.Code, tt.want, tt.wantConsuming, tt.wantCode)
			}
		})
	}
}
