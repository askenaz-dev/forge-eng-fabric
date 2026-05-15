package main

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RateLimiter interface {
	Allow(ctx context.Context, tenantID, workspaceID string) RateDecision
}

type RateDecision struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time
	Reason    string
}

type inMemoryRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*memBucket
	limit   int
	window  time.Duration
	clock   func() time.Time
}

type memBucket struct {
	count   int
	resetAt time.Time
}

func newInMemoryRateLimiter(limit int, window time.Duration) *inMemoryRateLimiter {
	return &inMemoryRateLimiter{buckets: map[string]*memBucket{}, limit: limit, window: window, clock: time.Now}
}

func (l *inMemoryRateLimiter) Allow(_ context.Context, tenant, workspace string) RateDecision {
	key := tenant + "|" + workspace
	now := l.clock()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok || now.After(b.resetAt) {
		b = &memBucket{resetAt: now.Add(l.window)}
		l.buckets[key] = b
	}
	if b.count >= l.limit {
		return RateDecision{Allowed: false, Limit: l.limit, ResetAt: b.resetAt, Reason: "rate_limit_exceeded"}
	}
	b.count++
	return RateDecision{Allowed: true, Limit: l.limit, Remaining: l.limit - b.count, ResetAt: b.resetAt}
}

type redisRateLimiter struct {
	addr   string
	limit  int
	window time.Duration
	dial   func(network, addr string) (net.Conn, error)
}

func newRedisRateLimiter(addr string, limit int, window time.Duration) *redisRateLimiter {
	return &redisRateLimiter{addr: addr, limit: limit, window: window, dial: net.Dial}
}

func (l *redisRateLimiter) Allow(ctx context.Context, tenant, workspace string) RateDecision {
	key := fmt.Sprintf("forge:a2a:rl:%s:%s:%d", tenant, workspace, time.Now().Truncate(l.window).Unix())
	conn, err := l.dial("tcp", l.addr)
	if err != nil {
		return RateDecision{Allowed: true, Limit: l.limit, Reason: "redis_unreachable_failopen"}
	}
	defer conn.Close()
	if d, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(d)
	} else {
		_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
	}
	pipe := strings.Join([]string{
		respCommand("INCR", key),
		respCommand("EXPIRE", key, strconv.Itoa(int(l.window.Seconds()))),
	}, "")
	if _, err := conn.Write([]byte(pipe)); err != nil {
		return RateDecision{Allowed: true, Limit: l.limit, Reason: "redis_write_failopen"}
	}
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return RateDecision{Allowed: true, Limit: l.limit, Reason: "redis_read_failopen"}
	}
	count := parseIntReply(string(buf[:n]))
	if count < 0 {
		return RateDecision{Allowed: true, Limit: l.limit, Reason: "redis_parse_failopen"}
	}
	if count > l.limit {
		return RateDecision{Allowed: false, Limit: l.limit, ResetAt: time.Now().Add(l.window), Reason: "rate_limit_exceeded"}
	}
	return RateDecision{Allowed: true, Limit: l.limit, Remaining: l.limit - count, ResetAt: time.Now().Add(l.window)}
}

func respCommand(args ...string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&sb, "$%d\r\n%s\r\n", len(a), a)
	}
	return sb.String()
}

func parseIntReply(s string) int {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return -1
	}
	end := strings.Index(s[idx:], "\r\n")
	if end < 0 {
		return -1
	}
	n, err := strconv.Atoi(s[idx+1 : idx+end])
	if err != nil {
		return -1
	}
	return n
}
