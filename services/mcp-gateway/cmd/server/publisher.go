package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"
)

// Publisher is the seam the gateway uses to emit invocation events.
// The production implementation talks to Kafka via the Sarama-free
// franz-go style "fire and forget"; for §4 (and to keep this service's
// dep graph small) we ship a tiny TCP-based stub that the operator can
// either wire to a real broker or replace via the Kafka client of their
// choice once observability infrastructure stabilizes.
//
// Tests inject a recording stub via the same interface.
type Publisher interface {
	Publish(ctx context.Context, eventType string, key, body []byte) error
}

// noopPublisher is the default when no broker is configured. Returns nil
// so the gateway works in local dev / tests without a Kafka broker.
type noopPublisher struct{}

func (noopPublisher) Publish(_ context.Context, _ string, _, _ []byte) error { return nil }

// tcpPublisher is a placeholder broker writer that connects to a remote
// address and writes the framed event. This is intentionally a stub —
// callers in production wire a real broker via Publisher seam. We keep
// it here so the wiring in main.go is exercised at build time.
type tcpPublisher struct {
	addr  string
	topic string
	mu    sync.Mutex
	dial  func(network, addr string) (net.Conn, error)
}

func (t *tcpPublisher) Publish(ctx context.Context, _ string, key, body []byte) error {
	if t.addr == "" {
		return errors.New("publisher: no broker addr configured")
	}
	conn, err := t.dial("tcp", t.addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	if d, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(d)
	} else {
		_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	// Wire-line framing: `topic <key_len> <key> <body_len> <body>\n`.
	// Real production wiring replaces this with a proper Kafka client.
	_, err = conn.Write([]byte(t.topic + "\t" + string(key) + "\t" + string(body) + "\n"))
	return err
}

func newPublisher(brokers, topic string) Publisher {
	if brokers == "" || brokers == "disabled" {
		return noopPublisher{}
	}
	addr := strings.Split(brokers, ",")[0]
	return &tcpPublisher{addr: addr, topic: topic, dial: net.Dial}
}
