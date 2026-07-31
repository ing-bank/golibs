package kafka

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	confluentkafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/ing-bank/golibs/pkg/kafka/producer"
	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
)

func newMockProducer(t *testing.T) *producer.Producer {
	t.Helper()

	cluster, err := confluentkafka.NewMockCluster(1)
	if err != nil {
		t.Fatalf("failed to create mock cluster: %v", err)
	}
	t.Cleanup(func() { cluster.Close() })

	client, err := producer.NewForConfig(
		&producer.Config{
			Brokers: []string{cluster.BootstrapServers()},
			Topic:   "store-kafka-middleware",
		},
		producer.WithDeliveryCh(make(chan confluentkafka.Event, 16)),
	)
	if err != nil {
		t.Fatalf("failed to create producer: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("failed to close producer: %v", err)
		}
	})

	return client
}

func newMockProducerWithCluster(t *testing.T, topic string) (*producer.Producer, *confluentkafka.MockCluster) {
	t.Helper()

	cluster, err := confluentkafka.NewMockCluster(1)
	if err != nil {
		t.Fatalf("failed to create mock cluster: %v", err)
	}
	t.Cleanup(func() { cluster.Close() })

	if err := cluster.CreateTopic(topic, 1, 1); err != nil {
		t.Fatalf("failed to create topic: %v", err)
	}

	client, err := producer.NewForConfig(
		&producer.Config{
			Brokers: []string{cluster.BootstrapServers()},
			Topic:   topic,
		},
		producer.WithDeliveryCh(make(chan confluentkafka.Event, 16)),
	)
	if err != nil {
		t.Fatalf("failed to create producer: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("failed to close producer: %v", err)
		}
	})

	return client, cluster
}

func nextDeliveryMessage(t *testing.T, bus *producer.Producer) Message[string, string] {
	t.Helper()

	select {
	case event := <-bus.DeliveryReports():
		msg, ok := event.(*confluentkafka.Message)
		if !ok {
			t.Fatalf("expected *kafka.Message, got %T", event)
		}
		if msg.TopicPartition.Error != nil {
			t.Fatalf("delivery report has error: %v", msg.TopicPartition.Error)
		}
		var payload Message[string, string]
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			t.Fatalf("failed to unmarshal produced message: %v", err)
		}
		return payload
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for delivery report")
		return Message[string, string]{}
	}
}

func assertNoDelivery(t *testing.T, bus *producer.Producer) {
	t.Helper()

	select {
	case event := <-bus.DeliveryReports():
		t.Fatalf("expected no delivery report, got %T", event)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestKafka_ProducesMessagesForWriteOperations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	backend := memory.NewOrDie[string, string]()
	bus := newMockProducer(t)

	db, err := New[string, string](backend, bus)
	if err != nil {
		t.Fatalf("unexpected error creating kafka middleware: %v", err)
	}

	if err := db.Create(ctx, "foo", "v1"); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	created := nextDeliveryMessage(t, bus)
	if created != (Message[string, string]{Key: "foo", Value: "v1", Action: ActionCreated}) {
		t.Fatalf("unexpected create payload: %+v", created)
	}

	if err := db.Update(ctx, "foo", "v2"); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	updated := nextDeliveryMessage(t, bus)
	if updated != (Message[string, string]{Key: "foo", Value: "v2", Action: ActionUpdated}) {
		t.Fatalf("unexpected update payload: %+v", updated)
	}

	if err := db.Apply(ctx, "bar", "v3"); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	applied := nextDeliveryMessage(t, bus)
	if applied != (Message[string, string]{Key: "bar", Value: "v3", Action: ActionApplied}) {
		t.Fatalf("unexpected apply payload: %+v", applied)
	}

	if err := db.Delete(ctx, "foo"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	deleted := nextDeliveryMessage(t, bus)
	if deleted.Action != ActionDeleted || deleted.Key != "foo" {
		t.Fatalf("unexpected delete payload: %+v", deleted)
	}
}

func TestKafka_DryRunBehavior(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("does not produce on dry run by default", func(t *testing.T) {
		backend := memory.NewOrDie[string, string]()
		bus := newMockProducer(t)

		db, err := New[string, string](backend, bus)
		if err != nil {
			t.Fatalf("unexpected error creating kafka middleware: %v", err)
		}

		if err := db.Create(ctx, "foo", "bar", store.DryRun); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		assertNoDelivery(t, bus)
	})

	t.Run("produces on dry run when enabled", func(t *testing.T) {
		backend := memory.NewOrDie[string, string]()
		bus := newMockProducer(t)

		db, err := New[string, string](backend, bus, Config{SendDryRun: true})
		if err != nil {
			t.Fatalf("unexpected error creating kafka middleware: %v", err)
		}

		if err := db.Create(ctx, "foo", "bar", store.DryRun); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		message := nextDeliveryMessage(t, bus)
		if message != (Message[string, string]{Key: "foo", Value: "bar", Action: ActionCreated}) {
			t.Fatalf("unexpected dry-run payload: %+v", message)
		}
	})
}

func TestKafka_DelegatesReadAndList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	backend := memory.NewOrDie[string, string]()
	if err := backend.Apply(ctx, "foo", "bar"); err != nil {
		t.Fatalf("failed to seed backend: %v", err)
	}
	bus := newMockProducer(t)

	db, err := New[string, string](backend, bus)
	if err != nil {
		t.Fatalf("unexpected error creating kafka middleware: %v", err)
	}

	value, err := db.Read(ctx, "foo")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if value != "bar" {
		t.Fatalf("unexpected read value: %q", value)
	}

	items, err := db.List(ctx)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(items) != 1 || items[0].Key != "foo" || items[0].Value != "bar" {
		t.Fatalf("unexpected list result: %+v", items)
	}
	assertNoDelivery(t, bus)
}

func TestKafka_DoesNotProduceWhenStoreOperationFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	backend := memory.NewOrDie[string, string]()
	bus := newMockProducer(t)

	db, err := New[string, string](backend, bus)
	if err != nil {
		t.Fatalf("unexpected error creating kafka middleware: %v", err)
	}

	if err := db.Update(ctx, "missing", "value"); err == nil {
		t.Fatal("expected update error for missing key")
	}
	assertNoDelivery(t, bus)
}

func TestKafka_WaitForDelivery(t *testing.T) {
	t.Parallel()

	const topic = "store-kafka-middleware-wait-delivery"
	ctx := context.Background()
	backend := memory.NewOrDie[string, string]()
	bus, cluster := newMockProducerWithCluster(t, topic)

	consumer, err := confluentkafka.NewConsumer(&confluentkafka.ConfigMap{
		"bootstrap.servers": cluster.BootstrapServers(),
		"group.id":          "store-kafka-middleware-wait-delivery-group",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		t.Fatalf("failed to create consumer: %v", err)
	}
	t.Cleanup(func() {
		if err := consumer.Close(); err != nil {
			t.Fatalf("failed to close consumer: %v", err)
		}
	})
	if err := consumer.Subscribe(topic, nil); err != nil {
		t.Fatalf("failed to subscribe to topic: %v", err)
	}

	db, err := New[string, string](backend, bus, Config{WaitForDelivery: true, WaitForDeliveryTimeout: 3 * time.Second})
	if err != nil {
		t.Fatalf("unexpected error creating kafka middleware: %v", err)
	}

	if err := db.Apply(ctx, "foo", "bar"); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	message, err := consumer.ReadMessage(5 * time.Second)
	if err != nil {
		t.Fatalf("failed to read consumed message: %v", err)
	}

	var payload Message[string, string]
	if err := json.Unmarshal(message.Value, &payload); err != nil {
		t.Fatalf("failed to unmarshal consumed payload: %v", err)
	}
	if payload != (Message[string, string]{Key: "foo", Value: "bar", Action: ActionApplied}) {
		t.Fatalf("unexpected consumed payload: %+v", payload)
	}
}

func TestIsDryRun(t *testing.T) {
	t.Parallel()

	if !IsDryRun([]store.Option{store.DryRun}) {
		t.Fatal("expected dry-run option to be detected")
	}
	if IsDryRun(nil) {
		t.Fatal("expected nil options to not be treated as dry-run")
	}
}

