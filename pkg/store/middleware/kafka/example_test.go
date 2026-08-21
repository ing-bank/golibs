package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	confluentkafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/ing-bank/golibs/pkg/kafka/producer"
	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
)

func Example() {
	ctx := context.Background()

	// Setup kafka stuff
	const topic = "store-kafka-middleware-example"
	cluster, _ := confluentkafka.NewMockCluster(1)
	defer cluster.Close()
	_ = cluster.CreateTopic(topic, 1, 1)

	bus, _ := producer.NewForConfig(
		&producer.Config{
			Brokers: []string{cluster.BootstrapServers()},
			Topic:   topic,
		},
		producer.WithDeliveryCh(make(chan confluentkafka.Event, 1)),
	)
	defer func() { _ = bus.Close() }()
	consumer, _ := confluentkafka.NewConsumer(&confluentkafka.ConfigMap{
		"bootstrap.servers": cluster.BootstrapServers(),
		"group.id":          "store-kafka-middleware-example-group",
		"auto.offset.reset": "earliest",
	})
	defer func() { _ = consumer.Close() }()
	_ = consumer.Subscribe(topic, nil)

	// Setup store with kafka middleware
	db, _ := store.New(
		memory.NewBuilder[string, string](),
		NewBuilder[string, string](bus, Config{WaitForDelivery: true}), // kafka middleware
	)

	// Create an entry
	_ = db.Apply(ctx, "foo", "bar")

	// Kafka consumer should receive message
	msg, _ := consumer.ReadMessage(5 * time.Second)
	var payload Message[string, string]
	_ = json.Unmarshal(msg.Value, &payload)
	fmt.Printf("received action=%s key=%s value=%s\n", payload.Action, payload.Key, payload.Value)

	// Output:
	// received action=applied key=foo value=bar
}
