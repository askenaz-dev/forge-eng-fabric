// Command forge-artifact-store invokes the artifact-store adapter from CI.
// It takes the per-Tenant backend config + credential as flags and runs a
// Put against the configured driver, printing the resulting artifact
// pointer (and the canonical digest) on stdout so the calling workflow
// can hand them to the registry's `lifecycle-hooks/gateway-publish` hook.
//
// Usage:
//
//	forge-artifact-store put \
//	  --backend nexus --tenant t1 \
//	  --base-url https://nexus.example.com \
//	  --credential-ref env://FORGE_NEXUS_USERPASS \
//	  --asset-id skill-foo --version 1.2.3 \
//	  --signature-id sha256:... \
//	  --attestation-id <id> \
//	  --in path/to/skill-foo-1.2.3.tar.zst
//
// The credential ref understands two schemes for portability:
//
//   - env://NAME      reads the secret bytes from $NAME (CI runner var)
//   - file://path     reads bytes from a local file (fixture / sealed
//                     workflow secret expanded into the workspace)
//
// In production deployment the registry server holds the BindingConfig
// from the DB and resolves the credential via Vault; this CLI exists so
// the CI pipeline can do the same job without going through the registry
// for the bytes themselves.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	adapter "github.com/forge-eng-fabric/pkg/artifact-store-adapter"
	_ "github.com/forge-eng-fabric/pkg/artifact-store-adapter/artifactory"
	_ "github.com/forge-eng-fabric/pkg/artifact-store-adapter/codeartifact"
	_ "github.com/forge-eng-fabric/pkg/artifact-store-adapter/ghpackages"
	_ "github.com/forge-eng-fabric/pkg/artifact-store-adapter/nexus"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: forge-artifact-store put [flags]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "put":
		os.Exit(runPut(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q (try: put)\n", os.Args[1])
		os.Exit(2)
	}
}

func runPut(args []string) int {
	fs := flag.NewFlagSet("put", flag.ContinueOnError)
	backend := fs.String("backend", "", "backend: nexus | artifactory | github-packages-private | codeartifact")
	tenant := fs.String("tenant", "", "tenant id (UUID string)")
	credRef := fs.String("credential-ref", "", "credential ref (env://NAME, file://path)")
	assetID := fs.String("asset-id", "", "asset id (id-scoped name)")
	version := fs.String("version", "", "asset version (SemVer)")
	in := fs.String("in", "", "path to the artifact bundle to upload")
	signatureID := fs.String("signature-id", "", "cosign signature reference")
	attestationID := fs.String("attestation-id", "", "in-toto attestation reference")
	settingsRaw := fs.String("settings", "", "extra backend settings as JSON (e.g. '{\"base_url\":\"...\"}')")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *backend == "" || *tenant == "" || *credRef == "" || *assetID == "" || *version == "" || *in == "" {
		fmt.Fprintln(os.Stderr, "all of --backend --tenant --credential-ref --asset-id --version --in are required")
		return 2
	}

	// Read the artifact bytes once so we can hash + replay.
	bytes, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *in, err)
		return 1
	}
	sum := sha256.Sum256(bytes)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	settings := map[string]any{}
	if *settingsRaw != "" {
		if err := json.Unmarshal([]byte(*settingsRaw), &settings); err != nil {
			fmt.Fprintf(os.Stderr, "decode --settings: %v\n", err)
			return 2
		}
	}

	secret, err := resolveCredential(*credRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "credential: %v\n", err)
		return 1
	}

	factory := adapter.NewFactory(
		adapter.StaticSecretFetcher{*credRef: secret},
		stdoutAuditSink{},
	)
	a, err := factory.Build(context.Background(), adapter.BindingConfig{
		TenantID:      *tenant,
		Backend:       *backend,
		CredentialRef: *credRef,
		Settings:      settings,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "build adapter: %v\n", err)
		return 1
	}

	obj := adapter.Object{
		TenantID: *tenant,
		AssetID:  *assetID,
		Version:  *version,
		Digest:   digest,
	}
	manifest := adapter.Manifest{
		AssetID:       *assetID,
		Version:       *version,
		Digest:        digest,
		SizeBytes:     int64(len(bytes)),
		SignatureID:   *signatureID,
		AttestationID: *attestationID,
	}
	if err := a.Put(context.Background(), obj, readerOf(bytes), int64(len(bytes)), manifest); err != nil {
		fmt.Fprintf(os.Stderr, "put: %v\n", err)
		return 1
	}

	// Emit a stable JSON line so the workflow's next step can parse it.
	out := map[string]any{
		"backend":         a.Backend(),
		"digest":          digest,
		"size_bytes":      len(bytes),
		"artifact_pointer": artifactPointer(a.Backend(), *tenant, *assetID, *version),
		"signature_id":    *signatureID,
		"attestation_id":  *attestationID,
	}
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(out)
	return 0
}

// resolveCredential supports the env:// and file:// schemes documented at
// the top of this file. Both schemes are sufficient for CI; production
// services use the SecretFetcher seam to integrate with Vault/IAM.
func resolveCredential(ref string) ([]byte, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "env":
		name := strings.TrimPrefix(strings.TrimPrefix(ref, "env://"), u.Host)
		if name == "" {
			name = u.Host
		}
		val, ok := os.LookupEnv(name)
		if !ok {
			return nil, fmt.Errorf("env var %q not set", name)
		}
		return []byte(val), nil
	case "file":
		path := strings.TrimPrefix(ref, "file://")
		return os.ReadFile(path)
	default:
		return nil, errors.New("credential-ref scheme must be env:// or file:// in CI")
	}
}

// artifactPointer returns the canonical pointer string the registry stores
// on active_surface_json for skill assets. The shape mirrors what each
// backend's REST surface uses for content addressing.
func artifactPointer(backend, tenant, assetID, version string) string {
	switch backend {
	case adapter.BackendNexus:
		return fmt.Sprintf("nexus://forge-skills-%s/%s/%s/%s-%s.tar.zst", tenant, assetID, version, assetID, version)
	case adapter.BackendArtifactory:
		return fmt.Sprintf("artifactory://forge-skills-%s/%s/%s/%s-%s.tar.zst", tenant, assetID, version, assetID, version)
	case adapter.BackendGitHubPackages:
		return fmt.Sprintf("github-packages://forge-skills-%s/skill/%s/%s", tenant, assetID, version)
	case adapter.BackendCodeArtifact:
		return fmt.Sprintf("codeartifact://forge-skills-%s/%s/%s/%s-%s.tar.zst", tenant, assetID, version, assetID, version)
	default:
		return fmt.Sprintf("%s://%s/%s/%s", backend, tenant, assetID, version)
	}
}

// readerOf returns an io.Reader over b without copying.
func readerOf(b []byte) io.Reader { return &byteReader{b: b} }

type byteReader struct{ b []byte }

func (r *byteReader) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.b)
	r.b = r.b[n:]
	return n, nil
}

// stdoutAuditSink writes audit events as one-line JSON to stderr so the
// workflow's logs capture them.
type stdoutAuditSink struct{}

func (stdoutAuditSink) EmitArtifactEvent(_ context.Context, e adapter.AuditEvent) error {
	b, _ := json.Marshal(e)
	fmt.Fprintln(os.Stderr, "audit:", string(b))
	return nil
}
