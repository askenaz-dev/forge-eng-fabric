// Package deployers defines the common `Deployer` interface that runtime
// connectors (GKE, Cloud Run, Minikube) implement, per the
// `runtime-connectors` spec.
package deployers

import (
	"context"
	"fmt"
	"strings"
	"time"

	rt "github.com/forge-eng-fabric/pkg/runtimes"
)

type Strategy string

const (
	StrategyRolling   Strategy = "rolling"
	StrategyCanary    Strategy = "canary"
	StrategyBlueGreen Strategy = "blue_green"
)

// Manifest carries the rendered application manifest material plus metadata.
type Manifest struct {
	AppName        string
	Namespace      string
	Image          string
	ImageDigest    string
	Replicas       int
	HelmChart      string
	HelmValues     map[string]any
	K8sYAML        string
	CloudRunImage  string
	HealthCheckURL string
	Env            map[string]string
	Labels         map[string]string
}

type Params struct {
	Strategy        Strategy
	CanaryPercent   int
	RolloutTimeout  time.Duration
	HealthTimeout   time.Duration
	RollbackPlan    string
	OpenSpecIDs     []string
	PRSHA           string
	RevisionID      string
	PrevRevisionID  string
}

type RenderedArtifacts struct {
	Files       map[string]string
	ManifestSHA string
	Notes       []string
}

type ApplyResult struct {
	Outcome       string
	RevisionID    string
	StageDuration time.Duration
	Detail        map[string]any
}

type VerifyResult struct {
	Healthy       bool
	StageDuration time.Duration
	Detail        map[string]any
	FailReason    string
}

type RollbackResult struct {
	Outcome       string
	RestoredRevID string
	StageDuration time.Duration
	Detail        map[string]any
}

type PreflightResult struct {
	Passed bool
	Reason string
	Detail map[string]any
}

// Deployer is the common interface that all runtime connectors implement.
type Deployer interface {
	Type() rt.Type
	Capabilities() rt.Capabilities
	Preflight(ctx context.Context, runtime *rt.Runtime) PreflightResult
	Render(ctx context.Context, manifest Manifest, params Params) (RenderedArtifacts, error)
	Apply(ctx context.Context, runtime *rt.Runtime, artifacts RenderedArtifacts, params Params) (ApplyResult, error)
	Verify(ctx context.Context, runtime *rt.Runtime, manifest Manifest, params Params) (VerifyResult, error)
	Rollback(ctx context.Context, runtime *rt.Runtime, prevManifest Manifest, params Params) (RollbackResult, error)
}

// Registry holds the connectors keyed by runtime type.
type Registry struct {
	deployers map[rt.Type]Deployer
}

func NewRegistry(ds ...Deployer) *Registry {
	r := &Registry{deployers: map[rt.Type]Deployer{}}
	for _, d := range ds {
		r.deployers[d.Type()] = d
	}
	return r
}

func (r *Registry) For(t rt.Type) (Deployer, error) {
	d, ok := r.deployers[t]
	if !ok {
		return nil, fmt.Errorf("no deployer registered for type=%s", t)
	}
	return d, nil
}

// CommandRunner abstracts shell-out for connectors. Production wires this to
// `os/exec`; tests substitute `FakeRunner` capturing every invocation.
type CommandRunner interface {
	Run(ctx context.Context, cmd string, args ...string) (stdout string, stderr string, err error)
}

// CommandLog is a single invocation captured by FakeRunner.
type CommandLog struct {
	Cmd  string
	Args []string
	Tag  string
}

type FakeRunner struct {
	Log     []CommandLog
	Stdout  map[string]string
	Stderr  map[string]string
	Errors  map[string]error
}

func NewFakeRunner() *FakeRunner {
	return &FakeRunner{Stdout: map[string]string{}, Stderr: map[string]string{}, Errors: map[string]error{}}
}

func (f *FakeRunner) Run(_ context.Context, cmd string, args ...string) (string, string, error) {
	tag := cmd + " " + strings.Join(args, " ")
	f.Log = append(f.Log, CommandLog{Cmd: cmd, Args: append([]string{}, args...), Tag: tag})
	for prefix, out := range f.Stdout {
		if strings.HasPrefix(tag, prefix) {
			return out, f.Stderr[prefix], f.Errors[prefix]
		}
	}
	return "", "", nil
}

func (f *FakeRunner) Calls(prefix string) []CommandLog {
	var out []CommandLog
	for _, c := range f.Log {
		if strings.HasPrefix(c.Tag, prefix) {
			out = append(out, c)
		}
	}
	return out
}
