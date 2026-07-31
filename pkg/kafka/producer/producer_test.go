package producer

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	confluentinckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry/serde"
	"github.com/ing-bank/golibs/pkg/kafka/stats"
)

type User struct {
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
}

func TestProducer(t *testing.T) {
	t.Parallel()
	cluster := SetupMockCluster(t)
	serde.MaybeFail = serde.InitFailFunc(t)

	client := MockProducer(t, cluster)

	topic := "topic"
	user := &User{
		Firstname: "John",
		Lastname:  "Doe",
	}

	_ = client.MonitorEvents(t.Context())
	// Allow some time for the monitor to start
	time.Sleep(2 * time.Second)

	payload, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("Error marshalling user: %v", err)
	}
	msg := NewMessage(payload, topic, confluentinckafka.Header{Key: "X-Unittest", Value: []byte("0001")})
	if err := client.Produce(msg); err != nil {
		t.Fatalf("Error running producer: %v", err)
	}

	// Wait for kafkaConsuming to process the message
	time.Sleep(4 * time.Second)

	status := client.Status()
	if status.Status != stats.UpState.String() {
		t.Fatalf("Expected status.Status to be 'UP', got '%s'", status.Status)
	}
	if status.Code != 0 {
		t.Fatalf("Expected status.Code to be 0, got %d", status.Code)
	}
	if status.Details.KafkaConsuming.Status != stats.UpState.String() {
		t.Fatalf("Expected status.Details.KafkaConsuming.Status to be 'UP', got '%s'", status.Details.KafkaConsuming.Status)
	}

	e := <-client.DeliveryReports()
	m := e.(*confluentinckafka.Message)

	newUser := &User{}
	if err := json.Unmarshal(m.Value, newUser); err != nil {
		t.Fatalf("Error unmarshalling user: %v", err)
	}
	if !reflect.DeepEqual(user, newUser) {
		t.Fatalf("Expected user to be %+v, got %+v", user, newUser)
	}
}

func TestProducer_WaitForDelivery(t *testing.T) {
	t.Parallel()

	cluster := SetupMockCluster(t)
	client := MockProducer(t, cluster)

	topic := "topic-wait-for-delivery"
	payload := []byte("wait-for-delivery")
	msg := NewMessage(payload, topic)

	if err := client.Produce(msg, WithWaitForDelivery(true)); err != nil {
		t.Fatalf("expected produce to wait and succeed, got error: %v", err)
	}

	select {
	case event := <-client.DeliveryReports():
		report, ok := event.(*confluentinckafka.Message)
		if !ok {
			t.Fatalf("expected *kafka.Message, got %T", event)
		}
		if report.TopicPartition.Error != nil {
			t.Fatalf("expected successful delivery report, got error: %v", report.TopicPartition.Error)
		}
		if !reflect.DeepEqual(report.Value, payload) {
			t.Fatalf("unexpected delivered payload: got %q want %q", report.Value, payload)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for forwarded delivery report")
	}
}

func TestParseProduceOptions_DefaultsAndTimeout(t *testing.T) {
	t.Parallel()

	options := NewProduceOptions()
	if err := WithWaitForDelivery(true)(options); err != nil {
		t.Fatalf("unexpected error applying wait option: %v", err)
	}
	if !options.WaitForDelivery {
		t.Fatal("expected wait for delivery to be enabled")
	}
	if options.WaitForDeliveryTimeout != 10*time.Second {
		t.Fatalf("unexpected default wait timeout: got %s, want %s", options.WaitForDeliveryTimeout, 10*time.Second)
	}

	options = NewProduceOptions()
	for _, option := range []ProduceOption{
		WithWaitForDelivery(true),
		WithWaitForDeliveryTimeout(1234 * time.Millisecond),
	} {
		if err := option(options); err != nil {
			t.Fatalf("unexpected error applying produce option: %v", err)
		}
	}
	if options.WaitForDeliveryTimeout != 1234*time.Millisecond {
		t.Fatalf("unexpected overridden wait timeout: got %s", options.WaitForDeliveryTimeout)
	}
}

func MockProducer(t *testing.T, cluster *confluentinckafka.MockCluster) *Producer {
	t.Helper()
	// Create a new Producer
	client, err := NewForConfig(&Config{
		Brokers: []string{cluster.BootstrapServers()},
		Topic:   "localonly",
	},
		WithDeliveryCh(make(chan confluentinckafka.Event, 1)),
	)
	if err != nil {
		t.Fatalf("Failed to create producer: %s", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("Failed to close producer: %s", err)
		}
	})
	return client
}

func SetupMockCluster(t *testing.T) *confluentinckafka.MockCluster {
	t.Helper()
	cluster, err := confluentinckafka.NewMockCluster(1)
	if err != nil {
		t.Fatalf("Failed to create mock cluster: %s", err)
	}
	t.Cleanup(func() {
		cluster.Close()
	})
	return cluster
}
