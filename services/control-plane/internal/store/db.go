package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgx pool with the queries we need.
type DB struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &DB{pool: pool}, nil
}

func (d *DB) Close()                         { d.pool.Close() }
func (d *DB) Ping(ctx context.Context) error { return d.pool.Ping(ctx) }

// --- models ------------------------------------------------------------

type Tenant struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type BusinessUnit struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Workspace struct {
	ID             uuid.UUID  `json:"id"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	BusinessUnitID uuid.UUID  `json:"business_unit_id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	Owners         []string   `json:"owners"`
	ArchivedAt     *time.Time `json:"archived_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type GitHubInstallation struct {
	ID             uuid.UUID `json:"id"`
	TenantID       uuid.UUID `json:"tenant_id"`
	WorkspaceID    uuid.UUID `json:"workspace_id"`
	InstallationID string    `json:"installation_id"`
	GitHubAccount  string    `json:"github_account"`
	Scopes         []string  `json:"scopes"`
	ConnectedAt    time.Time `json:"connected_at"`
	ConnectedBy    string    `json:"connected_by"`
}

// --- tenants ----------------------------------------------------------

func (d *DB) ListTenants(ctx context.Context) ([]Tenant, error) {
	rows, err := d.pool.Query(ctx, `SELECT id,name,created_at FROM tenant WHERE archived_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tenant{}
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *DB) CreateTenant(ctx context.Context, name, createdBy string) (*Tenant, error) {
	var t Tenant
	t.Name = name
	err := d.pool.QueryRow(ctx,
		`INSERT INTO tenant(name, created_by) VALUES ($1,$2) RETURNING id, created_at`,
		name, createdBy).Scan(&t.ID, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// --- business units --------------------------------------------------

func (d *DB) ListBUs(ctx context.Context, tenantID uuid.UUID) ([]BusinessUnit, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT id,tenant_id,name,created_at FROM business_unit
		 WHERE tenant_id=$1 AND archived_at IS NULL ORDER BY name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BusinessUnit{}
	for rows.Next() {
		var b BusinessUnit
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Name, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (d *DB) CreateBU(ctx context.Context, tenantID uuid.UUID, name, createdBy string) (*BusinessUnit, error) {
	var b BusinessUnit
	b.TenantID = tenantID
	b.Name = name
	err := d.pool.QueryRow(ctx,
		`INSERT INTO business_unit(tenant_id,name,created_by) VALUES ($1,$2,$3)
		 RETURNING id, created_at`,
		tenantID, name, createdBy).Scan(&b.ID, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// --- workspaces ------------------------------------------------------

func (d *DB) ListWorkspaces(ctx context.Context, buID *uuid.UUID) ([]Workspace, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if buID != nil {
		rows, err = d.pool.Query(ctx,
			`SELECT id,tenant_id,business_unit_id,name,COALESCE(description,''),owners,archived_at,created_at
			 FROM workspace WHERE business_unit_id=$1 AND archived_at IS NULL ORDER BY name`, *buID)
	} else {
		rows, err = d.pool.Query(ctx,
			`SELECT id,tenant_id,business_unit_id,name,COALESCE(description,''),owners,archived_at,created_at
			 FROM workspace WHERE archived_at IS NULL ORDER BY name`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Workspace{}
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.ID, &w.TenantID, &w.BusinessUnitID, &w.Name, &w.Description, &w.Owners, &w.ArchivedAt, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (d *DB) CreateWorkspace(ctx context.Context, buID uuid.UUID, name, desc string, owners []string, createdBy string) (*Workspace, error) {
	var w Workspace
	err := d.pool.QueryRow(ctx,
		`INSERT INTO workspace(tenant_id, business_unit_id, name, description, owners, created_by)
		 SELECT bu.tenant_id, bu.id, $2, $3, $4, $5 FROM business_unit bu WHERE bu.id=$1
		 RETURNING id, tenant_id, business_unit_id, name, COALESCE(description,''), owners, created_at`,
		buID, name, desc, owners, createdBy).
		Scan(&w.ID, &w.TenantID, &w.BusinessUnitID, &w.Name, &w.Description, &w.Owners, &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (d *DB) GetWorkspace(ctx context.Context, id uuid.UUID) (*Workspace, error) {
	var w Workspace
	err := d.pool.QueryRow(ctx,
		`SELECT id,tenant_id,business_unit_id,name,COALESCE(description,''),owners,archived_at,created_at
		 FROM workspace WHERE id=$1`, id).
		Scan(&w.ID, &w.TenantID, &w.BusinessUnitID, &w.Name, &w.Description, &w.Owners, &w.ArchivedAt, &w.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("workspace not found")
		}
		return nil, err
	}
	return &w, nil
}

func (d *DB) UpdateWorkspace(ctx context.Context, id uuid.UUID, name, desc *string, owners *[]string) (*Workspace, error) {
	// Build a small COALESCE-style update.
	_, err := d.pool.Exec(ctx,
		`UPDATE workspace SET
			name        = COALESCE($2, name),
			description = COALESCE($3, description),
			owners      = COALESCE($4, owners)
		 WHERE id=$1`,
		id, name, desc, owners)
	if err != nil {
		return nil, err
	}
	return d.GetWorkspace(ctx, id)
}

func (d *DB) ArchiveWorkspace(ctx context.Context, id uuid.UUID) (*Workspace, error) {
	_, err := d.pool.Exec(ctx, `UPDATE workspace SET archived_at = now() WHERE id=$1 AND archived_at IS NULL`, id)
	if err != nil {
		return nil, err
	}
	return d.GetWorkspace(ctx, id)
}

func (d *DB) CreateGitHubInstallation(ctx context.Context, workspaceID uuid.UUID, installationID, githubAccount string, scopes []string, connectedBy string) (*GitHubInstallation, error) {
	var g GitHubInstallation
	err := d.pool.QueryRow(ctx,
		`INSERT INTO github_installation(tenant_id, workspace_id, installation_id, github_account, scopes, connected_by)
		 SELECT w.tenant_id, w.id, $2, $3, $4, $5 FROM workspace w WHERE w.id=$1
		 RETURNING id, tenant_id, workspace_id, installation_id, github_account, scopes, connected_at, COALESCE(connected_by,'')`,
		workspaceID, installationID, githubAccount, scopes, connectedBy).
		Scan(&g.ID, &g.TenantID, &g.WorkspaceID, &g.InstallationID, &g.GitHubAccount, &g.Scopes, &g.ConnectedAt, &g.ConnectedBy)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (d *DB) LatestGitHubInstallation(ctx context.Context, workspaceID uuid.UUID) (*GitHubInstallation, error) {
	var g GitHubInstallation
	err := d.pool.QueryRow(ctx,
		`SELECT id, tenant_id, workspace_id, installation_id, github_account, scopes, connected_at, COALESCE(connected_by,'')
		 FROM github_installation
		 WHERE workspace_id=$1
		 ORDER BY connected_at DESC
		 LIMIT 1`, workspaceID).
		Scan(&g.ID, &g.TenantID, &g.WorkspaceID, &g.InstallationID, &g.GitHubAccount, &g.Scopes, &g.ConnectedAt, &g.ConnectedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("github installation not found")
		}
		return nil, err
	}
	return &g, nil
}
