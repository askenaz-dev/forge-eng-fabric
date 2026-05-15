package fingerprint

import (
	"testing"
)

func TestBuild_canonical(t *testing.T) {
	cases := []struct {
		name    string
		dims    Dims
		want    string
		wantErr bool
	}{
		{
			name: "required only",
			dims: Dims{"service": "workflow-registry", "signal": "probe-failed"},
			want: "service:workflow-registry|signal:probe-failed",
		},
		{
			name: "all optional sorted",
			dims: Dims{
				"service":     "workflow-registry",
				"signal":      "probe-failed",
				"error_class": "ECONNREFUSED",
				"port":        "8094",
			},
			want: "error_class:ECONNREFUSED|port:8094|service:workflow-registry|signal:probe-failed",
		},
		{
			name: "empty optional omitted",
			dims: Dims{"service": "svc", "signal": "log-pattern", "error_class": ""},
			want: "service:svc|signal:log-pattern",
		},
		{
			name:    "missing service",
			dims:    Dims{"signal": "probe-failed"},
			wantErr: true,
		},
		{
			name:    "missing signal",
			dims:    Dims{"service": "svc"},
			wantErr: true,
		},
		{
			name:    "unknown dimension",
			dims:    Dims{"service": "svc", "signal": "probe-failed", "hostname": "node1"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Build(tc.dims)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	valid := []string{
		"service:workflow-registry|signal:probe-failed",
		"error_class:ECONNREFUSED|port:8094|service:workflow-registry|signal:probe-failed",
		"service:kafka|signal:probe-failed|tenant:abc",
	}
	for _, fp := range valid {
		if err := Validate(fp); err != nil {
			t.Errorf("valid fingerprint %q failed: %v", fp, err)
		}
	}

	invalid := []struct {
		fp     string
		reason string
	}{
		{"", "empty"},
		{"signal:probe-failed", "missing service"},
		{"service:svc", "missing signal"},
		{"signal:probe-failed|service:svc", "not sorted"},
		{"service:svc|service:svc|signal:probe-failed", "duplicate dimension"},
		{"hostname:node1|service:svc|signal:probe-failed", "unknown dimension"},
		{"service:svc|signal:", "empty value"},
	}
	for _, tc := range invalid {
		if err := Validate(tc.fp); err == nil {
			t.Errorf("expected error for %q (%s), got nil", tc.fp, tc.reason)
		}
	}
}
