package codeartifact

// Minimal AWS SigV4 signer focused on the CodeArtifact REST API surface
// this driver exercises (POST/GET/DELETE on /v1/asset, /v1/package/...,
// /v1/domains). Implementing SigV4 in-house avoids pulling the
// aws-sdk-go-v2 dep tree into pkg/artifact-store-adapter, but the
// algorithm itself is the canonical one from the AWS docs:
// https://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html
//
// The signer supports:
//   - Static AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY pairs
//   - Optional AWS_SESSION_TOKEN (X-Amz-Security-Token header)
//
// Out of scope: STS AssumeRole, IMDS lookups, RoleArn config. Those run
// at the deployer / sidecar level; the secret fetcher hands us already-
// resolved credentials.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

type Signer struct {
	Creds   Credentials
	Region  string
	Service string // "codeartifact"
}

// Sign computes the SigV4 Authorization header and applies it (plus
// X-Amz-Date, X-Amz-Security-Token if a session token is set, and the
// content sha256) to req. For non-empty bodies, caller MUST pass the
// pre-computed body sha256 as bodyHash; for empty bodies pass
// "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855".
func (s *Signer) Sign(req *http.Request, bodyHash string, now time.Time) {
	now = now.UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	req.Header.Set("host", req.Host)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", bodyHash)
	if s.Creds.SessionToken != "" {
		req.Header.Set("x-amz-security-token", s.Creds.SessionToken)
	}

	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := canonicalQueryString(req.URL)

	signedHeaders, canonicalHeaders := canonicalizeHeaders(req)

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		bodyHash,
	}, "\n")

	credentialScope := dateStamp + "/" + s.Region + "/" + s.Service + "/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hashHex(canonicalRequest),
	}, "\n")

	kDate := hmacSHA256([]byte("AWS4"+s.Creds.SecretAccessKey), dateStamp)
	kRegion := hmacSHA256(kDate, s.Region)
	kService := hmacSHA256(kRegion, s.Service)
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	authz := "AWS4-HMAC-SHA256 Credential=" + s.Creds.AccessKeyID + "/" + credentialScope +
		", SignedHeaders=" + signedHeaders +
		", Signature=" + signature
	req.Header.Set("authorization", authz)
}

func canonicalizeHeaders(req *http.Request) (signedHeaders, canonicalHeaders string) {
	// SigV4 canonical headers: lowercased name, trimmed value, sorted by
	// name, joined with `\n`, terminated with `\n`. SignedHeaders is the
	// `;`-joined list of header names in the same order.
	names := make([]string, 0, len(req.Header)+1)
	hostSeen := false
	for k := range req.Header {
		lk := strings.ToLower(k)
		if lk == "authorization" {
			continue
		}
		if lk == "host" {
			hostSeen = true
		}
		names = append(names, lk)
	}
	if !hostSeen {
		names = append(names, "host")
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, n := range names {
		var val string
		if n == "host" {
			val = req.Host
		} else {
			val = strings.TrimSpace(req.Header.Get(n))
		}
		sb.WriteString(n)
		sb.WriteString(":")
		sb.WriteString(val)
		sb.WriteString("\n")
	}
	signedHeaders = strings.Join(names, ";")
	canonicalHeaders = sb.String()
	return
}

func canonicalQueryString(u *url.URL) string {
	if u.RawQuery == "" {
		return ""
	}
	parts := strings.Split(u.RawQuery, "&")
	pairs := make([][2]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		eq := strings.IndexByte(p, '=')
		var k, v string
		if eq < 0 {
			k, v = p, ""
		} else {
			k, v = p[:eq], p[eq+1:]
		}
		// Re-encode each component the SigV4 way.
		k = uriEncode(decodeOnce(k), false)
		v = uriEncode(decodeOnce(v), false)
		pairs = append(pairs, [2]string{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i][0] != pairs[j][0] {
			return pairs[i][0] < pairs[j][0]
		}
		return pairs[i][1] < pairs[j][1]
	})
	var sb strings.Builder
	for i, p := range pairs {
		if i > 0 {
			sb.WriteString("&")
		}
		sb.WriteString(p[0])
		sb.WriteString("=")
		sb.WriteString(p[1])
	}
	return sb.String()
}

func decodeOnce(s string) string {
	decoded, err := url.QueryUnescape(s)
	if err != nil {
		return s
	}
	return decoded
}

// uriEncode per SigV4 spec — unreserved characters stay literal,
// everything else is %HH-encoded. When encodePath=true, `/` is preserved.
func uriEncode(s string, encodePath bool) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		switch {
		case ('A' <= b && b <= 'Z') || ('a' <= b && b <= 'z') || ('0' <= b && b <= '9') ||
			b == '_' || b == '-' || b == '~' || b == '.':
			sb.WriteByte(b)
		case b == '/' && encodePath:
			sb.WriteByte(b)
		default:
			sb.WriteString("%")
			const hex = "0123456789ABCDEF"
			sb.WriteByte(hex[b>>4])
			sb.WriteByte(hex[b&0x0F])
		}
	}
	return sb.String()
}

func hashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	_, _ = m.Write([]byte(data))
	return m.Sum(nil)
}

// emptyBodySHA256 is the canonical sha256 of an empty body, used by GET
// and DELETE requests that carry no payload.
const emptyBodySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// HashBody reads and discards b's contents to compute the body sha256.
// Returns the hex digest and the buffered bytes so the caller can reuse
// them as the request body. Use only for small bodies; large uploads
// should pass the digest in directly to avoid double-buffering.
func HashBody(r io.Reader) (string, []byte, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), b, nil
}
