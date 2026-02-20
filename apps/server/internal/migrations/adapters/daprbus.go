package adapters

import (
	"context"
	"fmt"

	dapr "github.com/dapr/go-sdk/client"

	"github.com/tilsley/loom/apps/server/internal/migrations"
	"github.com/tilsley/loom/pkg/api"
)

// Compile-time check: *DaprBus implements migrations.WorkerNotifier.
var _ migrations.WorkerNotifier = (*DaprBus)(nil)

// DaprBus implements WorkerNotifier by publishing step requests to a Dapr pub/sub topic.
// Workers subscribe to this topic and execute the actual migration work.
type DaprBus struct {
	client     dapr.Client
	pubsubName string
	topic      string
}

// NewDaprBus creates a new DaprBus.
func NewDaprBus(client dapr.Client, pubsubName, topic string) *DaprBus {
	return &DaprBus{client: client, pubsubName: pubsubName, topic: topic}
}

// Dispatch publishes a step request to the Dapr pub/sub topic.
func (b *DaprBus) Dispatch(ctx context.Context, req api.DispatchStepRequest) error {
	if err := b.client.PublishEvent(ctx, b.pubsubName, b.topic, req); err != nil {
		return fmt.Errorf("publish to %s/%s: %w", b.pubsubName, b.topic, err)
	}
	return nil
}
