// Package ratelimit implements the per-PAT token bucket the gateway enforces
// in front of MCP / A2A / package routes. Backed by Redis so all gateway
// replicas share the bucket; degrades to local in-memory when Redis is not
// configured (dev / single-replica).
package ratelimit

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter has a single method, Allow, which returns whether the request can
// proceed and the Retry-After seconds when not.
type Limiter interface {
	Allow(ctx context.Context, key string) (bool, int, error)
}

// Config configures one limiter.
type Config struct {
	Capacity   int           // tokens per window
	Window     time.Duration // window length
}

// NewRedis returns a Redis-backed limiter using the simple INCR/EXPIRE
// pattern. Returns nil when client is nil so callers can opportunistically
// degrade.
func NewRedis(client *redis.Client, cfg Config) Limiter {
	if client == nil {
		return NewInMemory(cfg)
	}
	return &redisLimiter{client: client, cfg: cfg}
}

type redisLimiter struct {
	client *redis.Client
	cfg    Config
}

func (r *redisLimiter) Allow(ctx context.Context, key string) (bool, int, error) {
	bucket := key + ":" + strconv.FormatInt(time.Now().Truncate(r.cfg.Window).Unix(), 10)
	pipe := r.client.TxPipeline()
	incr := pipe.Incr(ctx, bucket)
	pipe.Expire(ctx, bucket, r.cfg.Window+time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, err
	}
	if incr.Val() > int64(r.cfg.Capacity) {
		retry := int(r.cfg.Window.Seconds())
		return false, retry, nil
	}
	return true, 0, nil
}

// NewInMemory is a process-local fallback. Not safe across replicas.
func NewInMemory(cfg Config) Limiter {
	return &inMemoryLimiter{cfg: cfg, counters: map[string]int{}}
}

type inMemoryLimiter struct {
	mu       sync.Mutex
	cfg      Config
	counters map[string]int
	resetAt  time.Time
}

func (m *inMemoryLimiter) Allow(_ context.Context, key string) (bool, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if now.After(m.resetAt) {
		m.counters = map[string]int{}
		m.resetAt = now.Add(m.cfg.Window)
	}
	m.counters[key]++
	if m.counters[key] > m.cfg.Capacity {
		return false, int(m.cfg.Window.Seconds()), nil
	}
	return true, 0, nil
}

// ErrRateLimited is returned by helpers that wrap Allow.
var ErrRateLimited = errors.New("rate_limited")
