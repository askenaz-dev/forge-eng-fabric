package ast

import (
	"errors"
	"testing"
)

func TestParseAssetRef(t *testing.T) {
	cases := []struct {
		in       string
		wantType string
		wantID   string
		wantVer  string
		wantErr  error
	}{
		{"registry:skill/sdlc-product/refine-user-story@1.2.0", "skill", "sdlc-product/refine-user-story", "1.2.0", nil},
		{"registry:mcp/github@1.0.0", "mcp", "github", "1.0.0", nil},
		{"registry:prompt/gen/x@2.3.4-rc.1", "prompt", "gen/x", "2.3.4-rc.1", nil},
		{"registry:skill/x/y@latest", "", "", "", ErrFloatingReference},
		{"skill/x/y@1.0.0", "", "", "", ErrNotRegistryRef},
		{"registry:skill/x/y", "", "", "", ErrMalformedReference},
	}
	for _, c := range cases {
		got, err := ParseAssetRef(c.in)
		if c.wantErr != nil {
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("%s: want %v, got %v", c.in, c.wantErr, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected err %v", c.in, err)
		}
		if got.Type != c.wantType || got.ID != c.wantID || got.Version != c.wantVer {
			t.Fatalf("%s: parsed %+v", c.in, got)
		}
	}
}
