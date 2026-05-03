package githubapp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var errRedisNil = errors.New("redis nil")

// RedisCache implements the minimal RESP commands we need without pulling in a full client SDK.
type RedisCache struct {
	addr     string
	username string
	password string
	db       int
	timeout  time.Duration
}

func NewRedisCache(rawURL string) (*RedisCache, error) {
	cache := &RedisCache{addr: rawURL, timeout: 2 * time.Second}
	if strings.Contains(rawURL, "://") {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("parse redis url: %w", err)
		}
		if u.Scheme != "redis" {
			return nil, fmt.Errorf("unsupported redis scheme %q", u.Scheme)
		}
		cache.addr = u.Host
		cache.username = u.User.Username()
		cache.password, _ = u.User.Password()
		if path := strings.TrimPrefix(u.Path, "/"); path != "" {
			db, err := strconv.Atoi(path)
			if err != nil {
				return nil, fmt.Errorf("parse redis db: %w", err)
			}
			cache.db = db
		}
	}
	if cache.addr == "" {
		return nil, errors.New("redis address required")
	}
	if _, _, err := net.SplitHostPort(cache.addr); err != nil {
		cache.addr = net.JoinHostPort(cache.addr, "6379")
	}
	return cache, nil
}

func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	data, err := c.do(ctx, "GET", key)
	if errors.Is(err, errRedisNil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	seconds := int(ttl.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	_, err := c.do(ctx, "SETEX", key, strconv.Itoa(seconds), string(value))
	return err
}

func (c *RedisCache) do(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	conn, err := (&net.Dialer{Timeout: c.timeout}).DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(c.timeout))

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	if c.password != "" {
		if c.username != "" {
			if _, err := redisCommand(rw, "AUTH", c.username, c.password); err != nil {
				return nil, err
			}
		} else if _, err := redisCommand(rw, "AUTH", c.password); err != nil {
			return nil, err
		}
	}
	if c.db != 0 {
		if _, err := redisCommand(rw, "SELECT", strconv.Itoa(c.db)); err != nil {
			return nil, err
		}
	}
	return redisCommand(rw, args...)
}

func redisCommand(rw *bufio.ReadWriter, args ...string) ([]byte, error) {
	if _, err := fmt.Fprintf(rw, "*%d\r\n", len(args)); err != nil {
		return nil, err
	}
	for _, arg := range args {
		if _, err := fmt.Fprintf(rw, "$%d\r\n", len(arg)); err != nil {
			return nil, err
		}
		if _, err := rw.WriteString(arg); err != nil {
			return nil, err
		}
		if _, err := rw.WriteString("\r\n"); err != nil {
			return nil, err
		}
	}
	if err := rw.Flush(); err != nil {
		return nil, err
	}
	return readRESP(rw.Reader)
}

func readRESP(r *bufio.Reader) ([]byte, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	switch prefix {
	case '+', ':':
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		return []byte(strings.TrimSpace(line)), nil
	case '-':
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		return nil, errors.New("redis: " + strings.TrimSpace(line))
	case '$':
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			return nil, err
		}
		if size == -1 {
			return nil, errRedisNil
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return buf[:size], nil
	default:
		return nil, fmt.Errorf("unexpected redis response prefix %q", prefix)
	}
}
