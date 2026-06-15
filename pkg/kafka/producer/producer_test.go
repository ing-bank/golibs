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
