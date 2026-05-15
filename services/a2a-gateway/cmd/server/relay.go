package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Relay forwards an A2A request to the resolved upstream and copies the
// response back. JSON-RPC responses are streamed through io.Copy; SSE
// streams for tasks/sendSubscribe are pumped through a bounded channel
// so caller backpressure propagates to the upstream agent.

type Relay struct {
	client *http.Client
}

func newRelay() *Relay {
	return &Relay{
		client: &http.Client{Transport: &http.Transport{
			MaxIdleConnsPerHost: 32,
			DisableCompression:  true,
		}},
	}
}

type RelayResult struct {
	Status    int
	BytesIn   int64
	BytesOut  int64
	Streaming bool
}

func (r *Relay) Forward(ctx context.Context, req *http.Request, w http.ResponseWriter, sseBufferSize int) (RelayResult, error) {
	req = req.WithContext(ctx)
	resp, err := r.client.Do(req)
	if err != nil {
		return RelayResult{}, fmt.Errorf("upstream call: %w", err)
	}
	defer resp.Body.Close()
	for k, vv := range resp.Header {
		if isHopByHop(k) {
			continue
		}
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	result := RelayResult{Status: resp.StatusCode}
	if isSSE(resp.Header.Get("content-type")) {
		bytesOut, err := r.streamSSE(ctx, resp.Body, w, sseBufferSize)
		result.BytesOut = bytesOut
		result.Streaming = true
		return result, err
	}
	n, err := io.Copy(w, resp.Body)
	result.BytesOut = n
	return result, err
}

func isSSE(ct string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToLower(ct)), "text/event-stream")
}

func isHopByHop(name string) bool {
	switch strings.ToLower(name) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade":
		return true
	}
	return false
}

func (r *Relay) streamSSE(ctx context.Context, src io.Reader, dst http.ResponseWriter, bufferSize int) (int64, error) {
	if bufferSize <= 0 {
		bufferSize = 16
	}
	flusher, ok := dst.(http.Flusher)
	if !ok {
		return 0, errors.New("relay: response writer does not implement http.Flusher")
	}
	type event struct{ data []byte }
	ch := make(chan event, bufferSize)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(src)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var sb []byte
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				if len(sb) == 0 {
					continue
				}
				out := make([]byte, len(sb)+1)
				copy(out, sb)
				out[len(sb)] = '\n'
				select {
				case ch <- event{data: out}:
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				}
				sb = sb[:0]
				continue
			}
			sb = append(sb, line...)
			sb = append(sb, '\n')
		}
		errCh <- scanner.Err()
	}()
	var bytesOut int64
	for {
		select {
		case <-ctx.Done():
			return bytesOut, ctx.Err()
		case e, ok := <-ch:
			if !ok {
				return bytesOut, <-errCh
			}
			n, err := dst.Write(e.data)
			bytesOut += int64(n)
			if err != nil {
				return bytesOut, err
			}
			flusher.Flush()
		}
	}
}

// drainAndCloseAfter is a small helper used by tests to clean up reader
// state when a test ends mid-stream.
func drainAndCloseAfter(rc io.ReadCloser, _ time.Duration) {
	if rc == nil {
		return
	}
	_, _ = io.Copy(io.Discard, rc)
	_ = rc.Close()
}
