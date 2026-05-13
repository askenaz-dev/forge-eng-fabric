package skillpackager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

// Reference is a remote document (or repo-relative path) the skill wants
// embedded under `references/` so the skill is usable offline.
type Reference struct {
	// URL is the source location. http(s) URLs are fetched; other schemes
	// must be expanded by the caller into a Body before calling Embed.
	URL string
	// PathInBundle is where the file should live in the bundle, relative
	// to `references/`. If empty, derived from path.Base of the URL.
	PathInBundle string
	// Body, when set, short-circuits the fetch. Tests and repo-relative
	// references use this.
	Body []byte
}

// IndexEntry is what `references/INDEX.json` records per embedded file.
type IndexEntry struct {
	Path   string `json:"path"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// Fetcher fetches a Reference's body. Use the default httpFetcher in prod;
// substitute in tests to avoid network.
type Fetcher interface {
	Fetch(ctx context.Context, url string) ([]byte, error)
}

type httpFetcher struct{ client *http.Client }

func newHTTPFetcher() *httpFetcher {
	return &httpFetcher{client: &http.Client{Timeout: 15 * time.Second}}
}

func (f *httpFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("reference_fetch_failed: GET %s -> %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// Embed turns the given references into [File] entries (under references/...)
// plus a references/INDEX.json describing them. The returned slice is meant
// to be appended to Spec.Files before calling Package.
func Embed(ctx context.Context, refs []Reference, fetcher Fetcher) ([]File, error) {
	if fetcher == nil {
		fetcher = newHTTPFetcher()
	}
	var files []File
	index := make([]IndexEntry, 0, len(refs))
	used := map[string]struct{}{}
	for _, r := range refs {
		if r.URL == "" && len(r.Body) == 0 {
			return nil, fmt.Errorf("reference must declare URL or Body")
		}
		body := r.Body
		if body == nil {
			if !strings.HasPrefix(r.URL, "http://") && !strings.HasPrefix(r.URL, "https://") {
				return nil, fmt.Errorf("reference URL %q is not http(s); pre-fetch and pass Body", r.URL)
			}
			b, err := fetcher.Fetch(ctx, r.URL)
			if err != nil {
				return nil, err
			}
			body = b
		}
		name := r.PathInBundle
		if name == "" {
			name = path.Base(r.URL)
			if name == "" || name == "." || name == "/" {
				return nil, fmt.Errorf("cannot derive filename from URL %q; set PathInBundle", r.URL)
			}
		}
		full := "references/" + strings.TrimPrefix(name, "/")
		if _, dup := used[full]; dup {
			return nil, fmt.Errorf("duplicate reference path %q", full)
		}
		used[full] = struct{}{}
		sum := sha256.Sum256(body)
		files = append(files, File{Path: full, Body: body})
		index = append(index, IndexEntry{
			Path:   full,
			URL:    r.URL,
			SHA256: "sha256:" + hex.EncodeToString(sum[:]),
		})
	}
	// Sort INDEX.json by path so it is deterministic regardless of input
	// order from the caller.
	indexBytes, err := json.MarshalIndent(stableIndex(index), "", "  ")
	if err != nil {
		return nil, err
	}
	files = append(files, File{Path: "references/INDEX.json", Body: append(indexBytes, '\n')})
	return files, nil
}

func stableIndex(in []IndexEntry) []IndexEntry {
	out := append([]IndexEntry(nil), in...)
	// path-sorted via simple insertion since slice is short.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].Path > out[j].Path; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
