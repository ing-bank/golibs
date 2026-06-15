package consumer

import (
	"errors"
	"os"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/spf13/pflag"
)

const (
	DefaultDryRun                 = false
	DefaultCommitInterval         = 15_000
	DefaultHeartbeatInterval      = 6_000
	DefaultMaxPollInterval        = 60_000
	DefaultSessionTimeout         = 30_000
	DefaultOffsetReset            = "earliest"
	DefaultStatisticsInterval     = 5_000
	DefaultSecurityProtocol       = "plaintext"
	DefaultSslKeyLocation         = ""
	DefaultSslCaLocation          = ""
	DefaultSslCertificateLocation = ""

	FlagKafkaConsumerTopics                 = "kafka-consumer-topics"
	FlagKafkaConsumerBrokers                = "kafka-consumer-brokers"
	FlagKafkaConsumerGroupID                = "kafka-consumer-group-id"
	FlagKafkaConsumerCommitInterval         = "kafka-consumer-commit-interval-ms"
	FlagKafkaConsumerHeartbeatInterval      = "kafka-consumer-heartbeat-interval-ms"
	FlagKafkaConsumerMaxPollInterval        = "kafka-consumer-max-poll-interval-ms"
	FlagKafkaConsumerSessionTimeout         = "kafka-consumer-session-timeout-ms"
	FlagKafkaConsumerStatisticsInterval     = "kafka-consumer-statistics-interval-ms"
	FlagKafkaConsumerDryRun                 = "kafka-consumer-dry-run"
	FlagKafkaConsumerAutoOffsetReset        = "kafka-consumer-auto-offset-reset"
	FlagKafkaConsumerSecurityProtocol       = "kafka-consumer-security-protocol"
	FlagKafkaConsumerSslCertificateLocation = "kafka-consumer-ssl-certificate-location"
	FlagKafkaConsumerSslKeyLocation         = "kafka-consumer-ssl-key-location"
	FlagKafkaConsumerSslCaLocation          = "kafka-consumer-ssl-ca-location"
)

// TLSConfig holds the configuration for a Kafka producer using TLS.
type TLSConfig struct {
	SecurityProtocol       string `yaml:"securityProtocol" json:"securityProtocol"`
	SslCertificateLocation string `yaml:"sslCertificateLocation" json:"sslCertificateLocation"`
	SslKeyLocation         string `yaml:"sslKeyLocation" json:"sslKeyLocation"`
	SslCaLocation          string `yaml:"sslCaLocation" json:"sslCaLocation"`
}

// Config holds the configuration for a Kafka consumer.
//
// Fields:
// - GroupID: The consumer group ID.
// - AutoOffsetReset: Policy for resetting offsets (e.g., "earliest", "latest").
// - DryRun: If true, messages are consumed but not committed.
// - Topics: List of topics to subscribe to.
// - CommitInterval: Interval (in millisecond) at which offsets are committed.
// - HeartbeatInterval: Interval (in millisecond) for sending heartbeats to the broker.
// - MaxPollInterval: Maximum interval (in millisecond) between poll requests.
// - SessionTimeout: Timeout (in millisecond) for consumer group sessions.
type Config struct {
	TLSConfig          `json:",inline" yaml:",inline"`
	GroupID            string   `json:"groupId" yaml:"groupId"`
	AutoOffsetReset    string   `json:"autoOffsetReset" yaml:"autoOffsetReset"`
	DryRun             bool     `json:"dryRun" yaml:"dryRun"`
	Topics             []string `json:"topics" yaml:"topics"`
	CommitInterval     int      `json:"commitInterval" yaml:"commitInterval"`
	HeartbeatInterval  int      `json:"heartbeatInterval" yaml:"heartbeatInterval"`
	MaxPollInterval    int      `json:"maxPollInterval" yaml:"maxPollInterval"`
	SessionTimeout     int      `json:"sessionTimeout" yaml:"sessionTimeout"`
	StatisticsInterval int      `json:"statisticsInterval" yaml:"statisticsInterval"`
	Brokers            []string `json:"brokers" yaml:"brokers"`
}

func DefaultConfig() *Config {
	c := new(Config)
	c.ApplyDefaults()
	return c
}

func (c *Config) ApplyDefaults() {
	ApplyDefaults(c)
}

// ApplyDefaults sets default values for the Config struct fields if they are not provided.
func ApplyDefaults(cfg *Config) *Config {
	if cfg.AutoOffsetReset == "" {
		cfg.AutoOffsetReset = DefaultOffsetReset
	}
	if cfg.CommitInterval <= 0 {
		cfg.CommitInterval = DefaultCommitInterval
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = DefaultHeartbeatInterval
	}
	if cfg.MaxPollInterval <= 0 {
		cfg.MaxPollInterval = DefaultMaxPollInterval
	}
	if cfg.SessionTimeout <= 0 {
		cfg.SessionTimeout = DefaultSessionTimeout
	}
	if cfg.StatisticsInterval <= 0 {
		cfg.StatisticsInterval = DefaultStatisticsInterval
	}
	if cfg.SecurityProtocol == "" {
		cfg.SecurityProtocol = DefaultSecurityProtocol
	}
	if cfg.SslKeyLocation == "" {
		cfg.SslKeyLocation = DefaultSslKeyLocation
	}
	if cfg.SslCaLocation == "" {
		cfg.SslCaLocation = DefaultSslCaLocation
	}
	if cfg.SslCertificateLocation == "" {
		cfg.SslCertificateLocation = DefaultSslCertificateLocation
	}

	// If Topics is not set, default it to the DefaultKafkaTopic
	return cfg
}

// ConfigMap converts the Config struct into a kafka.ConfigMap.
// The resulting ConfigMap can be used to configure a Kafka consumer.
//
// The following fields are included in the ConfigMap:
// - bootstrap.servers: A comma-separated list of broker addresses.
// - group.id: The consumer group ID.
// - session.timeout.ms: The timeout for consumer sessions.
// - auto.offset.reset: The offset reset policy (e.g., "earliest" or "latest").
// - enable.auto.commit: Whether to enable automatic offset commits (set to DryRun).
// - heartbeat.interval.ms: The interval between heartbeats to the broker.
// - auto.commit.interval.ms: The interval between automatic offset commits.
// - max.poll.interval.ms: The maximum interval between poll calls.
// - enable.auto.offset.store: Whether to enable automatic offset storage (set to false).
// - statistics.interval.ms: The interval for emitting statistics.
// - api.version.request: Whether to request the broker API version.
// - security.protocol: The security protocol to use (e.g., "SSL").
// - ssl.certificate.location: The location of the SSL certificate.
// - ssl.key.location: The location of the SSL key.
// - ssl.ca.location: The location of the SSL CA certificate.
//
// For more details on the configuration options, refer to the librdkafka documentation:
// https://github.com/confluentinc/librdkafka/blob/master/CONFIGURATION.md
func (c *Config) ConfigMap() *kafka.ConfigMap {
	return &kafka.ConfigMap{
		"bootstrap.servers":        strings.Join(c.Brokers, ","),
		"group.id":                 c.GroupID,
		"session.timeout.ms":       c.SessionTimeout,
		"auto.offset.reset":        c.AutoOffsetReset,
		"enable.auto.commit":       !c.DryRun,
		"heartbeat.interval.ms":    c.HeartbeatInterval,
		"auto.commit.interval.ms":  c.CommitInterval,
		"max.poll.interval.ms":     c.MaxPollInterval,
		"enable.auto.offset.store": false,
		"statistics.interval.ms":   c.StatisticsInterval,
		"security.protocol":        c.SecurityProtocol,
		"ssl.certificate.location": c.SslCertificateLocation,
		"ssl.key.location":         c.SslKeyLocation,
		"ssl.ca.location":          c.SslCaLocation,
	}
}

// Validate checks the Config for required fields and returns an error if any are missing.
func (c *Config) Validate() error {
	if len(c.Topics) == 0 {
		return errors.New("no topics defined")
	}
	if c.GroupID == "" {
		return errors.New("no group ID defined")
	}
	if len(c.Brokers) == 0 {
		return errors.New("no brokers defined")
	}
	return nil
}

func init() {
	if os.Getenv("PFLAGS_KAFKA_CONSUMER_ENABLED") == "1" {
		RegisterFlags(pflag.CommandLine)
	}
}

func RegisterFlags(flags *pflag.FlagSet) {
	if flags == nil {
		flags = pflag.CommandLine
	}
	c := DefaultConfig()
	flags.StringArray(FlagKafkaConsumerTopics, c.Topics, "Kafka topics to read the events from.")
	flags.StringArray(FlagKafkaConsumerBrokers, c.Brokers, "A list of host/port pairs to use for establishing the initial connection to the Kafka cluster.")
	flags.String(FlagKafkaConsumerGroupID, c.GroupID, "A unique string that identifies the consumer group this consumer belongs to.")
	flags.Int(FlagKafkaConsumerCommitInterval, c.CommitInterval, "The frequency in milliseconds that the consumer offsets are auto-committed to Kafka.")
	flags.Int(FlagKafkaConsumerHeartbeatInterval, c.HeartbeatInterval, "The expected time between heartbeats to the group coordinator when using Kafka's group management facilities.")
	flags.Int(FlagKafkaConsumerMaxPollInterval, c.MaxPollInterval, "This property specifies the maximum time allowed time between calls to the consumers poll method before the consumer process is assumed to have failed.")
	flags.Int(FlagKafkaConsumerSessionTimeout, c.SessionTimeout, "The timeout used to detect client failures when using Kafka's group management facility.")
	flags.Int(FlagKafkaConsumerStatisticsInterval, c.StatisticsInterval, "The interval to retrieve Kafka statistics.")
	flags.Bool(FlagKafkaConsumerDryRun, c.DryRun, "If true, messages are consumed but not committed.")
	flags.String(FlagKafkaConsumerAutoOffsetReset, c.AutoOffsetReset, "Action to take when there is no initial offset in offset store or the desired offset is out of range.")
	flags.String(FlagKafkaConsumerSecurityProtocol, c.SecurityProtocol, ".")
	flags.String(FlagKafkaConsumerSslCertificateLocation, c.SslCertificateLocation, "Path to client’s public key (PEM) used for authentication.")
	flags.String(FlagKafkaConsumerSslKeyLocation, c.SslKeyLocation, "Path to client’s private key (PEM) used for authentication.")
	flags.String(FlagKafkaConsumerSslCaLocation, c.SslCaLocation, "Path to CA certificate file used to verify the Schema Registry’s private key.")
}

// BindFlags binds the values from the given FlagSet to the Config fields.
func (c *Config) BindFlags(fs *pflag.FlagSet) error {
	if fs == nil {
		fs = pflag.CommandLine
	}
	var err error
	if fs.Changed(FlagKafkaConsumerTopics) {
		if c.Topics, err = fs.GetStringArray(FlagKafkaConsumerTopics); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaConsumerBrokers) {
		if c.Brokers, err = fs.GetStringArray(FlagKafkaConsumerBrokers); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaConsumerGroupID) {
		if c.GroupID, err = fs.GetString(FlagKafkaConsumerGroupID); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaConsumerCommitInterval) {
		if c.CommitInterval, err = fs.GetInt(FlagKafkaConsumerCommitInterval); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaConsumerHeartbeatInterval) {
		if c.HeartbeatInterval, err = fs.GetInt(FlagKafkaConsumerHeartbeatInterval); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaConsumerMaxPollInterval) {
		if c.MaxPollInterval, err = fs.GetInt(FlagKafkaConsumerMaxPollInterval); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaConsumerSessionTimeout) {
		if c.SessionTimeout, err = fs.GetInt(FlagKafkaConsumerSessionTimeout); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaConsumerStatisticsInterval) {
		if c.StatisticsInterval, err = fs.GetInt(FlagKafkaConsumerStatisticsInterval); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaConsumerDryRun) {
		if c.DryRun, err = fs.GetBool(FlagKafkaConsumerDryRun); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaConsumerAutoOffsetReset) {
		if c.AutoOffsetReset, err = fs.GetString(FlagKafkaConsumerAutoOffsetReset); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaConsumerSecurityProtocol) {
		if c.SecurityProtocol, err = fs.GetString(FlagKafkaConsumerSecurityProtocol); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaConsumerSslCertificateLocation) {
		if c.SslCertificateLocation, err = fs.GetString(FlagKafkaConsumerSslCertificateLocation); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaConsumerSslKeyLocation) {
		if c.SslKeyLocation, err = fs.GetString(FlagKafkaConsumerSslKeyLocation); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaConsumerSslCaLocation) {
		if c.SslCaLocation, err = fs.GetString(FlagKafkaConsumerSslCaLocation); err != nil {
			return err
		}
	}
	return nil
}
