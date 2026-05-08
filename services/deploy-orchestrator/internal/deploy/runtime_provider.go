package deploy

import (
	"context"
	"errors"

	rt "github.com/forge-eng-fabric/pkg/runtimes"
)

// RuntimeStoreProvider is a thin RuntimeProvider built on top of an
// in-process map of runtimes. The runtime-registry service exposes a similar
// interface; tests use this directly.
type RuntimeStoreProvider struct {
	runtimes map[string]*rt.Runtime
}

func NewRuntimeStoreProvider() *RuntimeStoreProvider {
	return &RuntimeStoreProvider{runtimes: map[string]*rt.Runtime{}}
}

func (p *RuntimeStoreProvider) Set(r *rt.Runtime) {
	p.runtimes[r.ID] = r
}

func (p *RuntimeStoreProvider) Get(_ context.Context, id string) (*rt.Runtime, error) {
	r, ok := p.runtimes[id]
	if !ok {
		return nil, errors.New("runtime_not_found")
	}
	return r, nil
}

func (p *RuntimeStoreProvider) CheckUsableBy(_ context.Context, id, workspaceID string) error {
	r, ok := p.runtimes[id]
	if !ok {
		return errors.New("runtime_not_found")
	}
	if r.Revoked {
		return ErrRuntimeRevoked
	}
	if r.WorkspaceID == workspaceID {
		return nil
	}
	if r.Visibility == rt.VisibilityTenant {
		return nil
	}
	return ErrCrossWorkspace
}
