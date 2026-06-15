package consumer

import (
	"reflect"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func TestApplyDefaults(t *testing.T) {
	type args struct {
		cfg *Config
	}
	tests := []struct {
		name string
		args args
		want *Config
	}{
		{
			name: "empty config returns defaulted fields",
			args: args{cfg: &Config{}},
			want: DefaultConfig(),
		},
		{
			name: "partial config returns mixed defaults",
			args: args{cfg: &Config{GroupID: "group1", Brokers: []string{"broker1:9092"}, Topics: []string{"topic1"}}},
			want: func() *Config {
				c := DefaultConfig()
				c.GroupID = "group1"
				c.Brokers = []string{"broker1:9092"}
				c.Topics = []string{"topic1"}
				return c
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ApplyDefaults(tt.args.cfg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ApplyDefaults() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_ConfigMap(t *testing.T) {
	type fields struct {
		TLSConfig          TLSConfig
		GroupID            string
		AutoOffsetReset    string
		DryRun             bool
		Topics             []string
		CommitInterval     int
		HeartbeatInterval  int
		MaxPollInterval    int
		SessionTimeout     int
		StatisticsInterval int
		Brokers            []string
	}
	tests := []struct {
		name   string
		fields fields
		want   *kafka.ConfigMap
	}{
		{
			name: "all fields set",
			fields: fields{
				TLSConfig: TLSConfig{
					SecurityProtocol:       "SSL",
					SslCertificateLocation: "/cert",
					SslKeyLocation:         "/key",
					SslCaLocation:          "/ca",
				},
				GroupID:            "group1",
				AutoOffsetReset:    "latest",
				DryRun:             true,
				Topics:             []string{"topic1"},
				CommitInterval:     1000,
				HeartbeatInterval:  2000,
				MaxPollInterval:    3000,
				SessionTimeout:     4000,
				StatisticsInterval: 5000,
				Brokers:            []string{"broker1:9092", "broker2:9092"},
			},
			want: &kafka.ConfigMap{
				"bootstrap.servers":        "broker1:9092,broker2:9092",
				"group.id":                 "group1",
				"session.timeout.ms":       4000,
				"auto.offset.reset":        "latest",
				"enable.auto.commit":       false,
				"heartbeat.interval.ms":    2000,
				"auto.commit.interval.ms":  1000,
				"max.poll.interval.ms":     3000,
				"enable.auto.offset.store": false,
				"statistics.interval.ms":   5000,
				"security.protocol":        "SSL",
				"ssl.certificate.location": "/cert",
				"ssl.key.location":         "/key",
				"ssl.ca.location":          "/ca",
			},
		},
		{
			name: "minimal config",
			fields: fields{
				TLSConfig: TLSConfig{},
				GroupID:   "group2",
				Brokers:   []string{"broker3:9092"},
			},
			want: &kafka.ConfigMap{
				"bootstrap.servers":        "broker3:9092",
				"group.id":                 "group2",
				"session.timeout.ms":       0,
				"auto.offset.reset":        "",
				"enable.auto.commit":       true,
				"heartbeat.interval.ms":    0,
				"auto.commit.interval.ms":  0,
				"max.poll.interval.ms":     0,
				"enable.auto.offset.store": false,
				"statistics.interval.ms":   0,
				"security.protocol":        "",
				"ssl.certificate.location": "",
				"ssl.key.location":         "",
				"ssl.ca.location":          "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				TLSConfig:          tt.fields.TLSConfig,
				GroupID:            tt.fields.GroupID,
				AutoOffsetReset:    tt.fields.AutoOffsetReset,
				DryRun:             tt.fields.DryRun,
				Topics:             tt.fields.Topics,
				CommitInterval:     tt.fields.CommitInterval,
				HeartbeatInterval:  tt.fields.HeartbeatInterval,
				MaxPollInterval:    tt.fields.MaxPollInterval,
				SessionTimeout:     tt.fields.SessionTimeout,
				StatisticsInterval: tt.fields.StatisticsInterval,
				Brokers:            tt.fields.Brokers,
			}
			if got := c.ConfigMap(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConfigMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	type fields struct {
		TLSConfig          TLSConfig
		GroupID            string
		AutoOffsetReset    string
		DryRun             bool
		Topics             []string
		CommitInterval     int
		HeartbeatInterval  int
		MaxPollInterval    int
		SessionTimeout     int
		StatisticsInterval int
		Brokers            []string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "valid config",
			fields: fields{
				GroupID: "group1",
				Topics:  []string{"topic1"},
				Brokers: []string{"broker1:9092"},
			},
			wantErr: false,
		},
		{
			name: "missing topics",
			fields: fields{
				GroupID: "group1",
				Brokers: []string{"broker1:9092"},
			},
			wantErr: true,
		},
		{
			name: "missing group id",
			fields: fields{
				Topics:  []string{"topic1"},
				Brokers: []string{"broker1:9092"},
			},
			wantErr: true,
		},
		{
			name: "missing brokers",
			fields: fields{
				GroupID: "group1",
				Topics:  []string{"topic1"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				TLSConfig:          tt.fields.TLSConfig,
				GroupID:            tt.fields.GroupID,
				AutoOffsetReset:    tt.fields.AutoOffsetReset,
				DryRun:             tt.fields.DryRun,
				Topics:             tt.fields.Topics,
				CommitInterval:     tt.fields.CommitInterval,
				HeartbeatInterval:  tt.fields.HeartbeatInterval,
				MaxPollInterval:    tt.fields.MaxPollInterval,
				SessionTimeout:     tt.fields.SessionTimeout,
				StatisticsInterval: tt.fields.StatisticsInterval,
				Brokers:            tt.fields.Brokers,
			}
			if err := c.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	tests := []struct {
		name string
		want *Config
	}{
		{
			name: "default config",
			want: DefaultConfig(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultConfig(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DefaultConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
