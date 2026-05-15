package adapter

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"strings"
)

// DigestSHA256 returns the canonical "sha256:<hex>" form of the bytes.
func DigestSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// VerifyDigest checks that observed matches expected. Returns nil on
// match, *Error with ErrCodeDigestMismatch on mismatch. The expected
// digest MUST be in canonical sha256:<hex> form.
func VerifyDigest(expected, observed string) error {
	if expected == "" {
		return &Error{Code: ErrCodeInvalidInput, Message: "expected digest empty"}
	}
	if !strings.HasPrefix(expected, "sha256:") {
		return &Error{Code: ErrCodeInvalidInput, Message: "expected digest not sha256: prefix"}
	}
	if expected != observed {
		return &Error{Code: ErrCodeDigestMismatch, Message: fmt.Sprintf("digest mismatch: expected=%s observed=%s", expected, observed)}
	}
	return nil
}

// HashingReader wraps an upstream io.Reader, hashing every byte read with
// sha256. Caller computes Sum() once the underlying reader is drained
// (typically EOF) to obtain the canonical digest. Useful for verifying
// uploads before committing.
type HashingReader struct {
	src io.Reader
	h   hash.Hash
}

func NewHashingReader(src io.Reader) *HashingReader {
	return &HashingReader{src: src, h: sha256.New()}
}

func (r *HashingReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	if n > 0 {
		_, _ = r.h.Write(p[:n])
	}
	return n, err
}

// Sum returns the canonical digest. Safe to call before EOF — it returns
// the digest of bytes observed so far. Drivers should only call Sum
// after the upstream reader returns io.EOF.
func (r *HashingReader) Sum() string {
	return "sha256:" + hex.EncodeToString(r.h.Sum(nil))
}

// VerifyingReadCloser wraps an io.ReadCloser. On Close, it compares the
// observed digest against the expected digest set at construction. If
// they differ, Close returns *Error with ErrCodeDigestMismatch — callers
// MUST check the error from Close before using the bytes downstream.
type VerifyingReadCloser struct {
	src      io.ReadCloser
	h        hash.Hash
	expected string
	closed   bool
}

// NewVerifyingReadCloser wraps src so that Close compares the streamed
// digest against expected. Pass the canonical sha256:<hex> form.
func NewVerifyingReadCloser(src io.ReadCloser, expected string) *VerifyingReadCloser {
	return &VerifyingReadCloser{src: src, h: sha256.New(), expected: expected}
}

func (v *VerifyingReadCloser) Read(p []byte) (int, error) {
	n, err := v.src.Read(p)
	if n > 0 {
		_, _ = v.h.Write(p[:n])
	}
	return n, err
}

func (v *VerifyingReadCloser) Close() error {
	if v.closed {
		return nil
	}
	v.closed = true
	if err := v.src.Close(); err != nil {
		return err
	}
	observed := "sha256:" + hex.EncodeToString(v.h.Sum(nil))
	return VerifyDigest(v.expected, observed)
}
