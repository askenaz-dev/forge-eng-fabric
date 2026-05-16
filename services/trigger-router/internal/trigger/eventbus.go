package trigger

import (
	"context"
	"log"
	"sync"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// BusSubscriber is the seam trigger-router uses to receive event-bus
// messages. Production wires Pulsar / NATS / Kafka; tests wire ChannelBus.
type BusSubscriber interface {
	Subscribe(topic string, handler func(BusMessage)) (cancel func(), err error)
}

// BusMessage is the in-process envelope. Production adapters convert
// their native frames into this shape before invoking the handler.
type BusMessage struct {
	Topic   string
	Payload map[string]any
}

// EventBusRouter dispatches incoming bus events to every subscribed
// trigger whose config.topic matches. Refuses to subscribe to topics not
// present in the platform event catalog (lint enforces this at publish,
// but the router defends in depth).
type EventBusRouter struct {
	Registry   *Registry
	Dispatcher *Dispatcher
	Bus        BusSubscriber
	KnownTopics map[string]struct{}

	mu      sync.Mutex
	cancels map[Key]func()
}

// NewEventBusRouter wires the registry, dispatcher, and bus adapter.
func NewEventBusRouter(reg *Registry, dispatch *Dispatcher, bus BusSubscriber, knownTopics map[string]struct{}) *EventBusRouter {
	return &EventBusRouter{
		Registry:    reg,
		Dispatcher:  dispatch,
		Bus:         bus,
		KnownTopics: knownTopics,
		cancels:     map[Key]func(){},
	}
}

// Refresh re-syncs bus subscriptions with the registry. Mirrors the
// CronScheduler.Refresh pattern: compute desired set, cancel removed,
// subscribe new.
func (r *EventBusRouter) Refresh() {
	r.mu.Lock()
	defer r.mu.Unlock()
	desired := map[Key]Subscription{}
	for _, sub := range r.Registry.ByType(ast.TriggerEventBus) {
		k := Key{sub.WorkflowID, sub.TriggerID, sub.Version}
		desired[k] = sub
	}
	// Removals.
	for k, cancel := range r.cancels {
		if _, ok := desired[k]; !ok {
			cancel()
			delete(r.cancels, k)
		}
	}
	// Additions.
	for k, sub := range desired {
		if _, ok := r.cancels[k]; ok {
			continue
		}
		topic, _ := sub.Config["topic"].(string)
		if topic == "" {
			log.Printf("event-bus: subscription %v missing config.topic", k)
			continue
		}
		if _, known := r.KnownTopics[topic]; !known {
			log.Printf("event-bus: topic %q not in catalog — refusing subscription for %v", topic, k)
			continue
		}
		sub := sub
		cancel, err := r.Bus.Subscribe(topic, func(msg BusMessage) {
			ctx := context.Background()
			_, ferr := r.Dispatcher.Fire(ctx, sub, msg.Payload)
			if ferr != nil {
				log.Printf("event-bus dispatch failed for %v: %v", k, ferr)
			}
		})
		if err != nil {
			log.Printf("event-bus subscribe failed for %v: %v", k, err)
			continue
		}
		r.cancels[k] = cancel
	}
}

// SubscribedCount returns how many bus subscriptions are live.
func (r *EventBusRouter) SubscribedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.cancels)
}

// ChannelBus is an in-process BusSubscriber for tests. Publish() injects
// a message; every matching subscriber's handler runs synchronously.
type ChannelBus struct {
	mu          sync.Mutex
	subscribers map[string][]func(BusMessage)
}

// NewChannelBus returns an empty ChannelBus.
func NewChannelBus() *ChannelBus {
	return &ChannelBus{subscribers: map[string][]func(BusMessage){}}
}

// Subscribe registers a handler for the topic; returns a cancel function.
func (b *ChannelBus) Subscribe(topic string, handler func(BusMessage)) (func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	idx := len(b.subscribers[topic])
	b.subscribers[topic] = append(b.subscribers[topic], handler)
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if idx < len(b.subscribers[topic]) {
			b.subscribers[topic] = append(b.subscribers[topic][:idx], b.subscribers[topic][idx+1:]...)
		}
	}, nil
}

// Publish delivers a message to every subscriber of its topic.
func (b *ChannelBus) Publish(topic string, payload map[string]any) {
	b.mu.Lock()
	handlers := append([]func(BusMessage){}, b.subscribers[topic]...)
	b.mu.Unlock()
	for _, h := range handlers {
		h(BusMessage{Topic: topic, Payload: payload})
	}
}
