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
	ID           uuid.UUID        `json:"id"`
	Name         string           `json:"name"`
	CreatedAt    time.Time        `json:"created_at"`
	FeatureFlags map[string]bool  `json:"feature_flags,omitempty"`
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

// GetTenantFeatureFlags returns the feature_flags map for the given tenant.
func (d *DB) GetTenantFeatureFlags(ctx context.Context, tenantID uuid.UUID) (map[string]bool, error) {
	var flags map[string]bool
	err := d.pool.QueryRow(ctx,
		`SELECT feature_flags FROM tenant WHERE id = $1`, tenantID).Scan(&flags)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("tenant not found")
		}
		return nil, err
	}
	if flags == nil {
		flags = map[string]bool{}
	}
	return flags, nil
}

// SetTenantFeatureFlags replaces the feature_flags map for the given tenant.
func (d *DB) SetTenantFeatureFlags(ctx context.Context, tenantID uuid.UUID, flags map[string]bool) error {
	_, err := d.pool.Exec(ctx,
		`UPDATE tenant SET feature_flags = $2 WHERE id = $1`, tenantID, flags)
	return err
}

// PatchTenantFeatureFlags merges patch into the existing feature_flags using
// jsonb || and returns the resulting map.
func (d *DB) PatchTenantFeatureFlags(ctx context.Context, tenantID uuid.UUID, patch map[string]bool) (map[string]bool, error) {
	var flags map[string]bool
	err := d.pool.QueryRow(ctx,
		`UPDATE tenant SET feature_flags = feature_flags || $2 WHERE id = $1 RETURNING feature_flags`,
		tenantID, patch).Scan(&flags)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("tenant not found")
		}
		return nil, err
	}
	if flags == nil {
		flags = map[string]bool{}
	}
	return flags, nil
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

// ListAllBUs returns every non-archived business unit, optionally including
// the tenant name for display. Caller-side FGA scoping applies in the handler.
func (d *DB) ListAllBUs(ctx context.Context) ([]BusinessUnit, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT id, tenant_id, name, created_at
		 FROM business_unit WHERE archived_at IS NULL ORDER BY name`)
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

// --- platform users --------------------------------------------------

type PlatformUser struct {
	Subject   string    `json:"subject"`
	Username  string    `json:"username,omitempty"`
	Email     string    `json:"email,omitempty"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// UpsertPlatformUser records that the given identity touched the platform.
// Safe to call on every authenticated request — keeps `last_seen` fresh and
// fills in username/email when the IdP supplies them.
func (d *DB) UpsertPlatformUser(ctx context.Context, subject, username, email string) error {
	if subject == "" {
		return nil
	}
	_, err := d.pool.Exec(ctx,
		`INSERT INTO platform_user(subject, username, email)
		 VALUES ($1, NULLIF($2,''), NULLIF($3,''))
		 ON CONFLICT (subject) DO UPDATE SET
		   username  = COALESCE(NULLIF(EXCLUDED.username,''),  platform_user.username),
		   email     = COALESCE(NULLIF(EXCLUDED.email,''),     platform_user.email),
		   last_seen = now()`,
		subject, username, email)
	return err
}

// ListPlatformUsers returns every identity ever observed by the auth
// middleware, ordered by username/email/subject for predictable display.
// `prefix`, when non-empty, narrows the result with a case-insensitive
// "starts-with" match against username or email.
func (d *DB) ListPlatformUsers(ctx context.Context, prefix string, limit int) ([]PlatformUser, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var rows pgx.Rows
	var err error
	if prefix == "" {
		rows, err = d.pool.Query(ctx,
			`SELECT subject, COALESCE(username,''), COALESCE(email,''), first_seen, last_seen
			 FROM platform_user
			 ORDER BY COALESCE(username, email, subject)
			 LIMIT $1`, limit)
	} else {
		pat := prefix + "%"
		rows, err = d.pool.Query(ctx,
			`SELECT subject, COALESCE(username,''), COALESCE(email,''), first_seen, last_seen
			 FROM platform_user
			 WHERE username ILIKE $1 OR email ILIKE $1
			 ORDER BY COALESCE(username, email, subject)
			 LIMIT $2`, pat, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlatformUser{}
	for rows.Next() {
		var u PlatformUser
		if err := rows.Scan(&u.Subject, &u.Username, &u.Email, &u.FirstSeen, &u.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
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
