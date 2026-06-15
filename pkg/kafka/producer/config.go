package producer

import (
	"errors"
	"os"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/spf13/pflag"
)

const (
	DefaultDryRun                 = false
	DefaultStatisticsInterval     = 5_000
	DefaultSecurityProtocol       = "plaintext"
	DefaultSslKeyLocation         = ""
	DefaultSslCaLocation          = ""
	DefaultSslCertificateLocation = ""

	FlagKafkaProducerTopic                  = "kafka-producer-topic"
	FlagKafkaProducerBrokers                = "kafka-producer-brokers"
	FlagKafkaProducerDryRun                 = "kafka-producer-dry-run"
	FlagKafkaProducerStatisticsInterval     = "kafka-producer-statistics-interval-ms"
	FlagKafkaProducerSecurityProtocol       = "kafka-producer-security-protocol"
	FlagKafkaProducerSslKeyLocation         = "kafka-producer-ssl-key-location"
	FlagKafkaProducerSslCaLocation          = "kafka-producer-ssl-ca-location"
	FlagKafkaProducerSslCertificateLocation = "kafka-producer-ssl-certificate-location"
)

// TLSConfig holds the configuration for a Kafka producer using TLS.
type TLSConfig struct {
	SecurityProtocol       string `yaml:"securityProtocol" json:"securityProtocol"`
	SslCertificateLocation string `yaml:"sslCertificateLocation" json:"sslCertificateLocation"`
	SslKeyLocation         string `yaml:"sslKeyLocation" json:"sslKeyLocation"`
	SslCaLocation          string `yaml:"sslCaLocation" json:"sslCaLocation"`
}

// Config holds the configuration for a Kafka producer.
type Config struct {
	Brokers            []string `yaml:"brokers" json:"brokers"`
	Topic              string   `yaml:"topic" json:"topic"`
	DryRun             bool     `yaml:"dryRun" json:"dryRun"`
	StatisticsInterval int      `yaml:"statisticsInterval" json:"statisticsInterval"` // Interval (in milliseconds) for collecting statistics.
	TLSConfig          `yaml:",inline" json:",inline"`
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
	// Set default values for the configuration fields if they are not provided
	if cfg.SecurityProtocol == "" {
		cfg.SecurityProtocol = DefaultSecurityProtocol
	}
	if cfg.SslKeyLocation == "" {
		cfg.SslKeyLocation = DefaultSslKeyLocation
	}
	if cfg.SslCaLocation == "" {
		cfg.SslCaLocation = DefaultSslCaLocation
	}
	if cfg.StatisticsInterval <= 0 {
		cfg.StatisticsInterval = DefaultStatisticsInterval
	}
	if cfg.SslCertificateLocation == "" {
		cfg.SslCertificateLocation = DefaultSslCertificateLocation
	}
	return cfg
}

// ConfigMap converts the Config struct into a kafka.ConfigMap.
// The resulting ConfigMap can be used to configure a Kafka producer.
//
// The following fields are included in the ConfigMap:
// - bootstrap.servers: A comma-separated list of broker addresses.
// - statistics.interval.ms: The interval for emitting statistics.
// - api.version.request: Whether to request the broker API version.
// - security.protocol: The security protocol to use (e.g., "SSL").
// - ssl.certificate.location: The location of the SSL certificate.
// - ssl.key.location: The location of the SSL key.
// - ssl.ca.location: The location of the SSL CA certificate.
// - enable.idempotence: Whether to enable idempotence for the producer (set to true).
//
// For more details on the configuration options, refer to the librdkafka documentation:
// https://github.com/confluentinc/librdkafka/blob/master/CONFIGURATION.md
func (c *Config) ConfigMap() *kafka.ConfigMap {
	return &kafka.ConfigMap{
		"bootstrap.servers":        strings.Join(c.Brokers, ","),
		"statistics.interval.ms":   c.StatisticsInterval,
		"security.protocol":        c.SecurityProtocol,
		"ssl.certificate.location": c.SslCertificateLocation,
		"ssl.key.location":         c.SslKeyLocation,
		"ssl.ca.location":          c.SslCaLocation,
		"enable.idempotence":       true,
	}
}

// Validate checks the Config for required fields and returns an error if any are missing.
func (c *Config) Validate() error {
	if c.Topic == "" {
		return errors.New("no topic defined")
	}
	if len(c.Brokers) == 0 {
		return errors.New("no brokers defined")
	}
	return nil
}

func init() {
	if os.Getenv("PFLAGS_KAFKA_PRODUCER_ENABLED") == "1" {
		RegisterFlags(pflag.CommandLine)
	}
}

func RegisterFlags(flags *pflag.FlagSet) {
	if flags == nil {
		flags = pflag.CommandLine
	}
	c := DefaultConfig()
	flags.String(FlagKafkaProducerTopic, c.Topic, "Kafka topic to deliver messages to.")
	flags.StringArray(FlagKafkaProducerBrokers, c.Brokers, "A list of host/port pairs to use for establishing the initial connection to the Kafka cluster.")
	flags.Bool(FlagKafkaProducerDryRun, c.DryRun, "If true, the producer will not send messages to Kafka.")
	flags.Int(FlagKafkaProducerStatisticsInterval, c.StatisticsInterval, "The interval to retrieve Kafka statistics.")
	flags.String(FlagKafkaProducerSecurityProtocol, c.SecurityProtocol, ".")
	flags.String(FlagKafkaProducerSslCertificateLocation, c.SslCertificateLocation, "Path to client’s public key (PEM) used for authentication.")
	flags.String(FlagKafkaProducerSslKeyLocation, c.SslKeyLocation, "Path to client’s private key (PEM) used for authentication.")
	flags.String(FlagKafkaProducerSslCaLocation, c.SslCaLocation, "Path to CA certificate file used to verify the Schema Registry’s private key.")
}

// BindFlags binds the values from the given FlagSet to the Config fields.
func (c *Config) BindFlags(fs *pflag.FlagSet) error {
	if fs == nil {
		fs = pflag.CommandLine
	}
	var err error
	if fs.Changed(FlagKafkaProducerTopic) {
		if c.Topic, err = fs.GetString(FlagKafkaProducerTopic); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaProducerBrokers) {
		if c.Brokers, err = fs.GetStringArray(FlagKafkaProducerBrokers); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaProducerDryRun) {
		if c.DryRun, err = fs.GetBool(FlagKafkaProducerDryRun); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaProducerStatisticsInterval) {
		if c.StatisticsInterval, err = fs.GetInt(FlagKafkaProducerStatisticsInterval); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaProducerSecurityProtocol) {
		if c.SecurityProtocol, err = fs.GetString(FlagKafkaProducerSecurityProtocol); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaProducerSslCertificateLocation) {
		if c.SslCertificateLocation, err = fs.GetString(FlagKafkaProducerSslCertificateLocation); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaProducerSslKeyLocation) {
		if c.SslKeyLocation, err = fs.GetString(FlagKafkaProducerSslKeyLocation); err != nil {
			return err
		}
	}
	if fs.Changed(FlagKafkaProducerSslCaLocation) {
		if c.SslCaLocation, err = fs.GetString(FlagKafkaProducerSslCaLocation); err != nil {
			return err
		}
	}
	return nil
}
