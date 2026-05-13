// Package skillpackager produces deterministic Agent Skills bundles from
// Forge Asset Registry metadata. A bundle is a folder named after the skill
// containing a `SKILL.md` (YAML front-matter + markdown body) and optional
// `scripts/`, `references/`, `assets/` directories, packed as a content-
// addressed `.tar.zst`.
//
// Determinism: for a given Spec the output bytes (and therefore the sha256)
// MUST be identical on every machine. Achieved by normalising mtime, uid/gid,
// file mode, header ordering and zstd encoder parameters.
//
// Signing and attestation are NOT performed here. Callers wire cosign /
// in-toto in the CI pipeline using the returned Result.Digest.
package skillpackager

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// EpochMTime is the fixed modification time written into every tar header so
// that bundles are reproducible across machines.
var EpochMTime = time.Unix(0, 0).UTC()

// Spec describes one skill bundle to produce. Fields beyond Name + Description
// are optional.
type Spec struct {
	// Name is the asset's registry name; used as the bundle's top-level
	// folder and as the front-matter `name` field. Must match
	// ^[a-z][a-z0-9-]{1,63}$.
	Name string
	// Description is the one-line summary the agent uses to decide when to
	// invoke the skill; goes into the front-matter `description`.
	Description string
	// Body is the markdown the skill author writes for the agent. The
	// packager prepends the YAML front-matter automatically.
	Body string
	// MCPDependencies lists registry MCP asset ids the skill needs at
	// install time. Surfaced as `mcp:` in the SKILL.md front-matter.
	MCPDependencies []string
	// Files contains the optional payload (scripts, references, assets).
	// Paths must be forward-slash relative paths. Symlinks, setuid bits,
	// and devices are rejected by the safety pass.
	Files []File
}

// File is a single in-bundle file. Mode is masked to 0644 / 0755 — only the
// owner-execute bit is preserved so scripts stay runnable.
type File struct {
	Path string
	Body []byte
	// Executable, when true, sets mode 0755 on the tar header. When false,
	// the file is written as 0644.
	Executable bool
}

// Result is what Package returns.
type Result struct {
	Bytes  []byte
	Digest string // hex-encoded sha256 of Bytes, prefixed with "sha256:"
	Size   int64
}

// Limits caps that Package enforces. Defaults match the spec.
type Limits struct {
	MaxCompressedBytes   int64
	MaxUncompressedBytes int64
}

// DefaultLimits — 50 MB compressed, 250 MB uncompressed.
var DefaultLimits = Limits{
	MaxCompressedBytes:   50 * 1024 * 1024,
	MaxUncompressedBytes: 250 * 1024 * 1024,
}

// Package builds a deterministic tar+zstd bundle from spec.
func Package(spec Spec, limits Limits) (*Result, error) {
	if limits.MaxCompressedBytes == 0 {
		limits = DefaultLimits
	}
	if err := validateSpec(spec); err != nil {
		return nil, err
	}
	if err := safetyCheck(spec); err != nil {
		return nil, err
	}

	manifest, err := renderManifest(spec)
	if err != nil {
		return nil, err
	}

	entries := []File{
		{Path: "SKILL.md", Body: manifest, Executable: false},
	}
	entries = append(entries, spec.Files...)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	tarBuf := &bytes.Buffer{}
	tw := tar.NewWriter(tarBuf)
	var uncompressed int64
	for _, e := range entries {
		full := spec.Name + "/" + e.Path
		mode := int64(0o644)
		if e.Executable {
			mode = 0o755
		}
		hdr := &tar.Header{
			Name:     full,
			Mode:     mode,
			Size:     int64(len(e.Body)),
			ModTime:  EpochMTime,
			Uid:      0,
			Gid:      0,
			Uname:    "",
			Gname:    "",
			Format:   tar.FormatPAX,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("tar header %s: %w", full, err)
		}
		if _, err := tw.Write(e.Body); err != nil {
			return nil, fmt.Errorf("tar body %s: %w", full, err)
		}
		uncompressed += int64(len(e.Body))
		if uncompressed > limits.MaxUncompressedBytes {
			return nil, fmt.Errorf("uncompressed size %d exceeds limit %d", uncompressed, limits.MaxUncompressedBytes)
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	out := &bytes.Buffer{}
	enc, err := zstd.NewWriter(out,
		zstd.WithEncoderLevel(zstd.SpeedDefault),
		zstd.WithEncoderConcurrency(1),
		zstd.WithWindowSize(1<<20),
	)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(enc, bytes.NewReader(tarBuf.Bytes())); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	body := out.Bytes()
	if int64(len(body)) > limits.MaxCompressedBytes {
		return nil, fmt.Errorf("compressed size %d exceeds limit %d", len(body), limits.MaxCompressedBytes)
	}
	sum := sha256.Sum256(body)
	return &Result{
		Bytes:  body,
		Digest: "sha256:" + hex.EncodeToString(sum[:]),
		Size:   int64(len(body)),
	}, nil
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !nameRE.MatchString(spec.Name) {
		return fmt.Errorf("name must match %s", nameRE.String())
	}
	if strings.TrimSpace(spec.Description) == "" {
		return fmt.Errorf("description is required")
	}
	seen := map[string]struct{}{}
	for _, f := range spec.Files {
		if f.Path == "" {
			return fmt.Errorf("file path is required")
		}
		if strings.Contains(f.Path, "\\") {
			return fmt.Errorf("file path %q must use forward slashes", f.Path)
		}
		if strings.HasPrefix(f.Path, "/") || strings.Contains(f.Path, "..") {
			return fmt.Errorf("file path %q must be a relative path under the bundle root", f.Path)
		}
		if _, dup := seen[f.Path]; dup {
			return fmt.Errorf("duplicate file path %q", f.Path)
		}
		seen[f.Path] = struct{}{}
	}
	return nil
}
