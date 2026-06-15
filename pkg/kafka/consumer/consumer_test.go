package consumer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	confluentinckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/ing-bank/golibs/pkg/kafka/producer"
)

var fakeError = fmt.Errorf("fake error")

var (
	payload = []byte("Hello World")
	topic   = "testonly"
)

func TestConsumerReadMessage(t *testing.T) {
	t.Parallel()
	cluster := SetupMockCluster(t)
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()

	MockProducer(t, cluster, payload)

	kafkaconsumer := MockConsumer(t, cluster, false)

	var found bool
	errChan := kafkaconsumer.HandleEvents(ctx, func(ctx context.Context, msg *confluentinckafka.Message) error {
		if !reflect.DeepEqual(payload, msg.Value) {
			return fmt.Errorf("expected message value to be '%s', got '%s'", payload, msg.Value)
		}
		found = true
		return nil
	})

	kafkaconsumer.Wait()

	if err := <-errChan; err.Err != nil {
		t.Fatalf("Consumer encountered an error: %s", err)
	}

	if !found {
		t.Fatalf("Failed to consume message")
	}
}

func TestConsumerTryingToReadTheSameMessageTwice(t *testing.T) {
	t.Parallel()
	cluster := SetupMockCluster(t)
	_ = MockProducer(t, cluster, payload)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	kafkaconsumer1 := MockConsumer(t, cluster, false)
	var msg1 []byte
	errCh1 := kafkaconsumer1.HandleEvents(ctx, func(ctx context.Context, msg *confluentinckafka.Message) error {
		msg1 = msg.Value
		return nil
	})
	if err := <-errCh1; err.Err != nil {
		t.Fatalf("Consumer1 encountered an error: %s", err)
	}
	if !reflect.DeepEqual(payload, msg1) {
		t.Fatalf("consumer1: expected message value to be '%s', got '%s'", payload, msg1)
	}

	if err := kafkaconsumer1.Close(); err != nil {
		t.Fatalf("Failed to close consumer: %s", err)
	}
	time.Sleep(3 * time.Second) // Wait for consumer group to close

	ctx2, cancel2 := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel2()

	kafkaconsumer2 := MockConsumer(t, cluster, false)
	var msg2 []byte
	errCh2 := kafkaconsumer2.HandleEvents(ctx2, func(ctx context.Context, msg *confluentinckafka.Message) error {
		msg2 = msg.Value
		return nil
	})
	if err := <-errCh2; err.Err != nil {
		t.Fatalf("Consumer2 encountered an error: %s", err)
	}
	if msg2 != nil {
		t.Fatalf("consumer2: expected message value to be nil, got '%s'", msg2)
	}
}

func TestConsumerTryingToReadACommittedMsg(t *testing.T) {
	t.Parallel()
	cluster := SetupMockCluster(t)
	_ = MockProducer(t, cluster, payload)

	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()

	kafkaconsumer1 := MockConsumer(t, cluster, false)
	var msg1 []byte
	errCh1 := kafkaconsumer1.HandleEvents(ctx, func(ctx context.Context, msg *confluentinckafka.Message) error {
		msg1 = msg.Value
		if err := kafkaconsumer1.CommitMessage(msg); err != nil {
			return fmt.Errorf("failed to commit message: %s", err)
		}
		return nil
	})
	if err := <-errCh1; err.Err != nil {
		t.Fatalf("Consumer1 encountered an error: %s", err)
	}
	if !reflect.DeepEqual(payload, msg1) {
		t.Fatalf("consumer1: expected message value to be '%s', got '%s'", payload, msg1)
	}

	if err := kafkaconsumer1.Close(); err != nil {
		t.Fatalf("Failed to close consumer: %s", err)
	}
	time.Sleep(3 * time.Second) // Wait for consumer group to close

	ctx2, cancel2 := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel2()

	kafkaconsumer2 := MockConsumer(t, cluster, false)
	var msg2 []byte
	errCh2 := kafkaconsumer2.HandleEvents(ctx2, func(ctx context.Context, msg *confluentinckafka.Message) error {
		msg2 = msg.Value
		return nil
	})
	if err := <-errCh2; err.Err != nil {
		t.Fatalf("Consumer2 encountered an error: %s", err)
	}
	if msg2 != nil {
		t.Fatalf("consumer2: expected message value to be nil, got '%s'", msg2)
	}
}

func TestConsumerReadingMsgTwiceWithDryRun(t *testing.T) {
	t.Parallel()
	cluster := SetupMockCluster(t)
	_ = MockProducer(t, cluster, payload)

	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()

	kafkaconsumer1 := MockConsumer(t, cluster, true)
	var msg1 []byte
	errCh1 := kafkaconsumer1.HandleEvents(ctx, func(ctx context.Context, msg *confluentinckafka.Message) error {
		msg1 = msg.Value
		return nil
	})
	if err := <-errCh1; err.Err != nil {
		t.Fatalf("Consumer1 encountered an error: %s", err)
	}
	if !reflect.DeepEqual(payload, msg1) {
		t.Fatalf("consumer1: expected message value to be '%s', got '%s'", payload, msg1)
	}

	if err := kafkaconsumer1.Close(); err != nil {
		t.Fatalf("Failed to close consumer: %s", err)
	}
	time.Sleep(3 * time.Second) // Wait for consumer group to close

	ctx2, cancel2 := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel2()

	kafkaconsumer2 := MockConsumer(t, cluster, false)
	var msg2 []byte
	errCh2 := kafkaconsumer2.HandleEvents(ctx2, func(ctx context.Context, msg *confluentinckafka.Message) error {
		msg2 = msg.Value
		return nil
	})
	if err := <-errCh2; err.Err != nil {
		t.Fatalf("Consumer2 encountered an error: %s", err)
	}
	if !reflect.DeepEqual(payload, msg2) {
		t.Fatalf("consumer2: expected message value to be '%s', got '%s'", payload, msg2)
	}
}

func TestConsumerProcessorWithErrors(t *testing.T) {
	t.Parallel()
	cluster := SetupMockCluster(t)
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()

	MockProducer(t, cluster, payload)

	kafkaconsumer := MockConsumer(t, cluster, false)
	errCh := kafkaconsumer.HandleEvents(ctx, func(ctx context.Context, msg *confluentinckafka.Message) error {
		return fakeError
	})

	kafkaconsumer.Wait()

	for err := range errCh {
		if !errors.Is(err.Err, fakeError) {
			t.Fatalf("Expected error to be '%s', got '%s'", fakeError, err)
		}

		parsedEv, ok := err.Event.(*confluentinckafka.Message)
		if !ok || !bytes.Equal(parsedEv.Value, []byte("Hello World")) {
			t.Fatalf("Expected event to be 'hello world', got '%s'", parsedEv.Value)
		}
	}

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

func MockProducer(t *testing.T, cluster *confluentinckafka.MockCluster, message []byte) *producer.Producer {
	t.Helper()
	var deliveryCh = make(chan confluentinckafka.Event)
	kafkaproducer, err := producer.NewForConfig(&producer.Config{
		Brokers: []string{cluster.BootstrapServers()},
		Topic:   topic,
	}, producer.WithDeliveryCh(deliveryCh))
	if err != nil {
		t.Fatalf("Failed to create producer: %s", err)
	}
	t.Cleanup(func() {
		if err := kafkaproducer.Close(); err != nil {
			t.Fatalf("Failed to close producer: %s", err)
		}
	})

	err = cluster.CreateTopic(topic, 1, 1) // Create a topic with 1 partition and 1 replica
	if err != nil {
		t.Fatalf("Failed to create topic: %s", err)
	}

	if message != nil && len(message) > 0 {
		msg := kafkaproducer.NewMessage(message)
		err = kafkaproducer.Produce(msg)
		if err != nil {
			t.Fatalf("Failed to produce message: %s", err)
		}
	}

	return kafkaproducer
}

func MockConsumer(t *testing.T, cluster *confluentinckafka.MockCluster, dryRun bool) *Consumer {
	t.Helper()

	var consumerConfig = &Config{
		AutoOffsetReset:   "earliest",
		DryRun:            dryRun,
		GroupID:           "test-group",
		HeartbeatInterval: 1_000,
		MaxPollInterval:   4_000,
		SessionTimeout:    4_000,
		CommitInterval:    4_000,
		Brokers:           []string{cluster.BootstrapServers()},
		Topics:            []string{topic},
	}

	kafkaconsumer, err := NewForConfig(consumerConfig)
	if err != nil {
		t.Fatalf("Failed to create consumer: %s", err)
	}
	t.Cleanup(func() {
		if !kafkaconsumer.IsClosed() {
			if err := kafkaconsumer.Close(); err != nil {
				t.Fatalf("Failed to close consumer: %s", err)
			}
		}
	})
	return kafkaconsumer
}
