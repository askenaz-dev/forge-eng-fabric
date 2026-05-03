package auth

import "testing"

func TestAudienceMatches(t *testing.T) {
	tests := []struct {
		name string
		aud  any
		want bool
	}{
		{name: "string match", aud: "forge-control-plane", want: true},
		{name: "string miss", aud: "other", want: false},
		{name: "array match", aud: []any{"account", "forge-control-plane"}, want: true},
		{name: "array miss", aud: []any{"account"}, want: false},
		{name: "nil", aud: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := audienceMatches(tt.aud, "forge-control-plane"); got != tt.want {
				t.Fatalf("audienceMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}
