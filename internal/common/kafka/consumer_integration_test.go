package kafka

import (
	"context"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/atlaspay/platform/pkg/events"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

func TestConsumerRetainedEvents(t *testing.T) {
	if os.Getenv("ATLASPAY_INTEGRATION") != "1" {
		t.Skip("set ATLASPAY_INTEGRATION=1 to run against Kafka")
	}
	broker := os.Getenv("ATLASPAY_TEST_KAFKA_BROKER")
	if broker == "" {
		broker = "kafka:29092"
	}
	topic := "atlaspay.consumer-test." + uuid.New().String()
	group := topic + ".group"
	transport := &kafka.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	client := &kafka.Client{Addr: kafka.TCP(broker), Timeout: 10 * time.Second, Transport: transport}
	setupCtx, setupCancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer setupCancel()
	metadata, err := client.Metadata(setupCtx, &kafka.MetadataRequest{})
	if err != nil {
		t.Fatalf("discover controller: %v", err)
	}
	controller := kafka.TCP(net.JoinHostPort(metadata.Controller.Host, strconv.Itoa(metadata.Controller.Port)))
	created, err := client.CreateTopics(setupCtx, &kafka.CreateTopicsRequest{
		Addr:   controller,
		Topics: []kafka.TopicConfig{{Topic: topic, NumPartitions: 1, ReplicationFactor: 1}},
	})
	if err != nil {
		t.Fatalf("create isolated topic: %v", err)
	}
	if err := created.Errors[topic]; err != nil {
		t.Fatalf("create isolated topic: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		deleted, err := client.DeleteTopics(ctx, &kafka.DeleteTopicsRequest{Addr: controller, Topics: []string{topic}})
		if err != nil {
			t.Errorf("delete isolated topic: %v", err)
		} else if err := deleted.Errors[topic]; err != nil {
			t.Errorf("delete isolated topic: %v", err)
		}
	})

	producer := NewProducer([]string{broker})
	t.Cleanup(func() {
		if err := producer.Close(); err != nil {
			t.Errorf("close producer: %v", err)
		}
	})
	var handled []string
	handler := EventHandlerFunc(func(_ context.Context, event *events.Event) error {
		handled = append(handled, event.ID)
		return nil
	})
	var consumer *Consumer
	t.Cleanup(func() {
		if consumer != nil {
			if err := consumer.Close(); err != nil {
				t.Errorf("close consumer: %v", err)
			}
		}
	})

	for index, name := range []string{"first", "second"} {
		event, err := events.NewEvent(events.OrderCreated, topic, topic, events.OrderCreatedPayload{OrderID: topic})
		if err != nil {
			t.Fatal(err)
		}
		event.ID = topic + "." + name
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		if err := producer.Publish(ctx, topic, event); err != nil {
			cancel()
			t.Fatalf("prepublish %s event: %v", name, err)
		}
		cancel()

		// Publish before joining: a new group must see retained events, and a
		// restarted group must resume after its committed first event.
		consumer = NewConsumerWithOptions([]string{broker}, topic, group, handler, nil, nil)
		ctx, cancel = context.WithTimeout(t.Context(), 30*time.Second)
		message, err := consumer.reader.FetchMessage(ctx)
		if err != nil {
			cancel()
			t.Fatalf("fetch retained %s event: %v", name, err)
		}
		processed, err := consumer.processMessage(ctx, message)
		cancel()
		if err != nil {
			t.Fatalf("handle and commit %s event: %v", name, err)
		}
		if processed == nil || processed.ID != event.ID || message.Offset != int64(index) || len(handled) != index+1 || handled[index] != event.ID {
			t.Fatalf("expected %s event at offset %d once; processed=%+v offset=%d handled=%v", name, index, processed, message.Offset, handled)
		}
		if err := consumer.Close(); err != nil {
			t.Fatalf("close %s consumer before restart: %v", name, err)
		}
		consumer = nil
	}
}
