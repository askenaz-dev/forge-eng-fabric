package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"
)

type Publisher interface {
	Publish(ctx context.Context, eventType string, key, body []byte) error
}

type noopPublisher struct{}

func (noopPublisher) Publish(_ context.Context, _ string, _, _ []byte) error { return nil }

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
