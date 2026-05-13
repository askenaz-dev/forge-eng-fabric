package skillpackager

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestPackage_DeterministicDigest(t *testing.T) {
	spec := Spec{
		Name:        "generate-test-cases",
		Description: "Generate test cases for an OpenSpec",
		Body:        "# generate-test-cases\n\nGiven an OpenSpec, propose test cases.",
		Files: []File{
			{Path: "scripts/run.sh", Body: []byte("#!/usr/bin/env bash\necho hi\n"), Executable: true},
		},
	}
	r1, err := Package(spec, DefaultLimits)
	if err != nil {
		t.Fatalf("first package: %v", err)
	}
	r2, err := Package(spec, DefaultLimits)
	if err != nil {
		t.Fatalf("second package: %v", err)
	}
	if r1.Digest != r2.Digest {
		t.Fatalf("digest is not deterministic: %s vs %s", r1.Digest, r2.Digest)
	}
	if !bytes.Equal(r1.Bytes, r2.Bytes) {
		t.Fatalf("bytes are not deterministic (len %d vs %d)", len(r1.Bytes), len(r2.Bytes))
	}
	if !strings.HasPrefix(r1.Digest, "sha256:") {
		t.Fatalf("digest must be sha256-prefixed, got %s", r1.Digest)
	}
}

func TestPackage_RefusesSecretPaths(t *testing.T) {
	spec := Spec{
		Name:        "leaky-skill",
		Description: "should fail",
		Files: []File{
			{Path: "scripts/.env", Body: []byte("API_KEY=hello")},
		},
	}
	if _, err := Package(spec, DefaultLimits); err == nil {
		t.Fatal("expected secret_material_in_source error, got nil")
	} else if !strings.Contains(err.Error(), "secret_material_in_source") {
		t.Fatalf("expected secret error, got %v", err)
	}
}

func TestPackage_RejectsPathEscape(t *testing.T) {
	spec := Spec{
		Name:        "escape-skill",
		Description: "should fail",
		Files: []File{
			{Path: "../etc/passwd", Body: []byte("nope")},
		},
	}
	if _, err := Package(spec, DefaultLimits); err == nil {
		t.Fatal("expected path validation error, got nil")
	}
}

func TestRenderManifest_FrontMatter(t *testing.T) {
	body, err := renderManifest(Spec{
		Name:            "x",
		Description:     "y",
		MCPDependencies: []string{"github"},
		Body:            "# x",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.HasPrefix(got, "---\n") {
		t.Fatalf("manifest must begin with front-matter, got %q", got)
	}
	for _, want := range []string{"name: x", "description: y", "mcp:", "- github", "# x"} {
		if !strings.Contains(got, want) {
			t.Fatalf("manifest missing %q. full:\n%s", want, got)
		}
	}
}

type fakeFetcher map[string][]byte

func (f fakeFetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	b, ok := f[url]
	if !ok {
		return nil, &fetcherError{url: url}
	}
	return b, nil
}

type fetcherError struct{ url string }

func (e *fetcherError) Error() string { return "reference_fetch_failed: " + e.url }

func TestEmbed_BuildsIndex(t *testing.T) {
	refs := []Reference{
		{URL: "https://docs.example/spec.md", PathInBundle: "spec.md"},
	}
	files, err := Embed(context.Background(), refs, fakeFetcher{
		"https://docs.example/spec.md": []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files (ref + INDEX.json), got %d", len(files))
	}
	var sawIndex, sawRef bool
	for _, f := range files {
		if f.Path == "references/spec.md" {
			sawRef = true
		}
		if f.Path == "references/INDEX.json" {
			sawIndex = true
			if !bytes.Contains(f.Body, []byte("\"sha256:")) {
				t.Fatalf("INDEX.json must record sha256, got %s", string(f.Body))
			}
		}
	}
	if !sawRef || !sawIndex {
		t.Fatal("expected both reference file and INDEX.json")
	}
}
