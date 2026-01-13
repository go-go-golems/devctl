package tui

import (
	"context"
	"sync"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	gochannel "github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/pkg/errors"
)

type Bus struct {
	Router     *message.Router
	Publisher  message.Publisher
	Subscriber message.Subscriber

	runOnce sync.Once
}

func NewInMemoryBus() (*Bus, error) {
	logger := watermill.NopLogger{}
	pubsub := gochannel.NewGoChannel(gochannel.Config{OutputChannelBuffer: 1024}, logger)

	r, err := message.NewRouter(message.RouterConfig{}, logger)
	if err != nil {
		return nil, errors.Wrap(err, "new watermill router")
	}
	return &Bus{
		Router:     r,
		Publisher:  pubsub,
		Subscriber: pubsub,
	}, nil
}

func (b *Bus) AddHandler(name, topic string, handler func(*message.Message) error) {
	b.Router.AddConsumerHandler(name, topic, b.Subscriber, handler)
}

func (b *Bus) Run(ctx context.Context) error {
	var runErr error
	b.runOnce.Do(func() {
		go func() {
			<-ctx.Done()
			_ = b.Router.Close()
		}()
		runErr = b.Router.Run(ctx)
	})
	return runErr
}
