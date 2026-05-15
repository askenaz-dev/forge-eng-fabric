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

// Relay handles the outbound MCP call. It supports two response shapes:
//
//  1. Plain HTTP — the upstream returns a single JSON response which we
//     mirror to the caller.
//  2. SSE — the upstream returns `text/event-stream` and we relay events
//     to the caller with end-to-end backpressure (one bounded channel
//     between upstream reader and caller writer, so a slow caller stalls
//     the upstream reader rather than buffering unboundedly).
//
// The relay never reads the response body into memory in full; HTTP bodies
// are streamed through io.Copy and SSE events are pumped line-by-line.
type Relay struct {
	client *http.Client
}

func newRelay() *Relay {
	return &Relay{
		client: &http.Client{
			Timeout: 0, // overall request timeout is enforced via per-call ctx
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 32,
				DisableCompression:  true, // SSE relies on non-buffered transport
			},
		},
	}
}

// Forward issues req to the upstream and copies the response to w. SSE
// responses are detected by Content-Type and relayed in streaming mode;
// non-SSE responses are copied through io.Copy.
//
// Returns BytesIn / BytesOut and the upstream status code so the audit /
// metrics layer can emit them. On caller disconnect (ctx.Done()) the
// upstream stream is closed, which propagates as connection close to the
// MCP — keeping backpressure end-to-end.
func (r *Relay) Forward(ctx context.Context, req *http.Request, w http.ResponseWriter, sseBufferSize int) (RelayResult, error) {
	req = req.WithContext(ctx)
	resp, err := r.client.Do(req)
	if err != nil {
		return RelayResult{}, fmt.Errorf("upstream call: %w", err)
	}
	defer resp.Body.Close()
	// Propagate the upstream status and headers verbatim. We drop hop-by-
	// hop headers per RFC 7230 §6.1.
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

// RelayResult is the post-call metadata the caller emits as audit /
// metrics.
type RelayResult struct {
	Status    int
	BytesIn   int64
	BytesOut  int64
	Streaming bool
}

func isSSE(contentType string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToLower(contentType)), "text/event-stream")
}

func isHopByHop(name string) bool {
	switch strings.ToLower(name) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade":
		return true
	}
	return false
}

// streamSSE relays SSE events from src to dst with end-to-end
// backpressure. We buffer at most `bufferSize` events between the
// upstream reader and the caller writer; once full the reader blocks,
// which back-propagates the slow consumer to the upstream MCP.
//
// On ctx.Done() the goroutine returns; src.Close() (deferred by the
// caller) terminates the upstream connection.
func (r *Relay) streamSSE(ctx context.Context, src io.Reader, dst http.ResponseWriter, bufferSize int) (int64, error) {
	if bufferSize <= 0 {
		bufferSize = 16
	}
	flusher, ok := dst.(http.Flusher)
	if !ok {
		return 0, errors.New("relay: response writer does not implement http.Flusher; SSE requires flush")
	}

	type event struct {
		data []byte
		ts   time.Time
	}
	ch := make(chan event, bufferSize)
	errCh := make(chan error, 1)

	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(src)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var sb []byte
		for scanner.Scan() {
			line := scanner.Bytes()
			// Empty line marks the end of an SSE event; flush the
			// accumulated lines to the channel.
			if len(line) == 0 {
				if len(sb) == 0 {
					continue
				}
				out := make([]byte, len(sb)+1)
				copy(out, sb)
				out[len(sb)] = '\n'
				select {
				case ch <- event{data: out, ts: time.Now()}:
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
		if err := scanner.Err(); err != nil {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	var bytesOut int64
	for {
		select {
		case <-ctx.Done():
			return bytesOut, ctx.Err()
		case e, ok := <-ch:
			if !ok {
				// Channel closed: reader finished; await its error.
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
