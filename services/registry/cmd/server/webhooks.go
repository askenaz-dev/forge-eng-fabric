package main

// webhooks.go — inbound npm and GitHub webhook receivers (Task 13.7)
//
// POST /v1/registry/webhooks/npm    — npm publish hooks
// POST /v1/registry/webhooks/github — GitHub release events
//
// Both endpoints:
//  1. Validate HMAC-SHA256 signature from X-Npm-Signature-256 / X-Hub-Signature-256
//  2. Parse payload for package name + version
//  3. Check if we track a public-origin asset for that package
//  4. Trigger Mirror() if the version is not already stored
//  5. Deduplicate: skip if lifecycle_state=mirrored already for this version
//
// If WEBHOOK_SECRET is unset all webhook payloads are rejected with 501.

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
)

// webhookSecret returns the shared secret used for HMAC validation.
// Returns "" when WEBHOOK_SECRET is unset.
func webhookSecret() string {
	return os.Getenv("WEBHOOK_SECRET")
}

// validateHMACSHA256 verifies an X-*-Signature-256 header of the form
// "sha256=<hex>" against the request body using the provided secret.
func validateHMACSHA256(body []byte, signatureHeader, secret string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}
	expectedHex := signatureHeader[len(prefix):]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	got := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(got), []byte(expectedHex))
}

// npmWebhookHandler handles POST /v1/registry/webhooks/npm.
//
// npm publish webhooks use X-Npm-Signature (sha1) and X-Npm-Signature-256
// (sha256). We verify the sha256 variant.
// Payload: https://docs.npmjs.com/about-registry-webhooks
func (s *server) npmWebhookHandler(w http.ResponseWriter, r *http.Request) {
	secret := webhookSecret()
	if secret == "" {
		http.Error(w, "webhook secret not configured", http.StatusNotImplemented)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB max
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("X-Npm-Signature-256")
	if sig == "" {
		// Older npm hooks send X-Npm-Signature (sha1). We require sha256.
		http.Error(w, "missing X-Npm-Signature-256 header", http.StatusBadRequest)
		return
	}
	if !validateHMACSHA256(body, sig, secret) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse npm publish payload.
	// npm send an envelope: {"event":"package:publish","name":"<pkg>","change":{"version":"<ver>",...}}
	var payload struct {
		Event  string `json:"event"`
		Name   string `json:"name"`
		Change struct {
			Version string `json:"version"`
		} `json:"change"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	if payload.Event != "package:publish" {
		// Not a publish event — acknowledge without action.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	pkg := payload.Name
	version := payload.Change.Version
	if pkg == "" || version == "" {
		http.Error(w, "missing package name or version in payload", http.StatusBadRequest)
		return
	}

	originRef := fmt.Sprintf("npm:%s@%s", pkg, version)
	s.handleIncomingWebhookVersion(r, originRef, pkg, version)
	w.WriteHeader(http.StatusNoContent)
}

// githubWebhookHandler handles POST /v1/registry/webhooks/github.
//
// GitHub signs with X-Hub-Signature-256: sha256=<hex>.
// We listen for the "release" event (X-GitHub-Event: release) with action="published".
func (s *server) githubWebhookHandler(w http.ResponseWriter, r *http.Request) {
	secret := webhookSecret()
	if secret == "" {
		http.Error(w, "webhook secret not configured", http.StatusNotImplemented)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20)) // 4 MiB max
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("X-Hub-Signature-256")
	if sig == "" {
		http.Error(w, "missing X-Hub-Signature-256 header", http.StatusBadRequest)
		return
	}
	if !validateHMACSHA256(body, sig, secret) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// Only handle "release" event type.
	if eventType := r.Header.Get("X-GitHub-Event"); eventType != "release" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Parse GitHub release payload.
	// https://docs.github.com/en/webhooks/webhook-events-and-payloads#release
	var payload struct {
		Action  string `json:"action"`
		Release struct {
			TagName string `json:"tag_name"`
		} `json:"release"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	if payload.Action != "published" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	repo := payload.Repository.FullName      // e.g. "owner/repo"
	tag := payload.Release.TagName           // e.g. "v1.2.3"
	if repo == "" || tag == "" {
		http.Error(w, "missing repository or tag in payload", http.StatusBadRequest)
		return
	}

	// Normalise version: strip leading "v" prefix if present.
	version := strings.TrimPrefix(tag, "v")
	originRef := fmt.Sprintf("github:%s@%s", repo, version)
	s.handleIncomingWebhookVersion(r, originRef, repo, version)
	w.WriteHeader(http.StatusNoContent)
}

// handleIncomingWebhookVersion checks whether we track an asset for the given
// package/repo and triggers Mirror() if a new version has been published.
//
// originRef is the canonical reference (e.g. "npm:pkg@1.2.3").
// pkg is the bare package name or GitHub "owner/repo" slug used to look up
// matching asset rows by their origin_ref prefix.
// version is the newly released semver string.
func (s *server) handleIncomingWebhookVersion(r *http.Request, originRef, pkg, version string) {
	ctx := r.Context()

	// Find assets that track this package. We match on origin_ref containing
	// the package name (before the @version suffix) so any tracked version of
	// the package is found.
	//
	// origin_ref is stored as "<scheme>:<pkg>@<ver>"; the prefix up to "@" is
	// the stable package identity across versions.
	pkgPrefix := originRef[:strings.LastIndex(originRef, "@")+1] // e.g. "npm:my-pkg@"

	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT ON (id) id, tenant_id, version, lifecycle_state, auto_promote_policy
		   FROM asset
		  WHERE is_public_origin = true
		    AND origin_ref LIKE $1
		  ORDER BY id, created_at DESC`,
		pkgPrefix+"%")
	if err != nil {
		log.Printf("webhooks: query assets for %s: %v", pkg, err)
		return
	}
	defer rows.Close()

	type assetRow struct {
		id                string
		tenantID          uuid.UUID
		storedVersion     string
		lifecycleState    string
		autoPromotePolicy string
	}

	var candidates []assetRow
	for rows.Next() {
		var row assetRow
		if scanErr := rows.Scan(&row.id, &row.tenantID, &row.storedVersion, &row.lifecycleState, &row.autoPromotePolicy); scanErr != nil {
			log.Printf("webhooks: scan asset row: %v", scanErr)
			continue
		}
		candidates = append(candidates, row)
	}
	if err := rows.Err(); err != nil {
		log.Printf("webhooks: iterate asset rows: %v", err)
		return
	}

	for _, row := range candidates {
		// Deduplication: skip if we already have this version mirrored.
		var existing int
		if err := s.pool.QueryRow(ctx,
			`SELECT count(*) FROM asset WHERE id=$1 AND version=$2`,
			row.id, version).Scan(&existing); err != nil {
			log.Printf("webhooks: check existing version %s@%s: %v", row.id, version, err)
			continue
		}
		if existing > 0 {
			log.Printf("webhooks: %s@%s already stored, skipping", row.id, version)
			continue
		}

		// Trigger Mirror for the new version.
		mirrorReq := MirrorRequest{
			OriginRef:           originRef,
			SHA256:              "", // Mirror() computes digest from fetched bytes
			TenantID:            row.tenantID,
			AssetID:             row.id,
			Version:             version,
			PublicOriginEnabled: true,
		}
		if _, mirrorErr := s.Mirror(ctx, mirrorReq); mirrorErr != nil {
			log.Printf("webhooks: mirror %s@%s: %v", row.id, version, mirrorErr)
		} else {
			log.Printf("webhooks: mirrored %s@%s from webhook", row.id, version)
		}
	}
}
