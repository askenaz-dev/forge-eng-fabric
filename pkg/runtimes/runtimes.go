// Package runtimes contains the shared public types for runtime descriptors,
// kept in a standalone module so that the runtime-registry service, the
// deployer connectors, and the deploy-orchestrator can all reference the
// same Runtime/Capabilities shapes without depending on one another's
// internal/ packages.
package runtimes

import "time"

type Type string

const (
	TypeGKE      Type = "gke"
	TypeCloudRun Type = "cloudrun"
	TypeMinikube Type = "minikube"
)

type Mode string

const (
	ModeBYO         Mode = "byo"
	ModeProvisioned Mode = "provisioned"
)

type Visibility string

const (
	VisibilityWorkspace Visibility = "workspace"
	VisibilityTenant    Visibility = "tenant"
)

type GKEMode string

const (
	GKEStandard  GKEMode = "standard"
	GKEAutopilot GKEMode = "autopilot"
)

type Capabilities struct {
	SupportsCanary           bool `json:"supports_canary"`
	SupportsBlueGreen        bool `json:"supports_blue_green"`
	SupportsSecretsCSI       bool `json:"supports_secrets_csi"`
	SupportsTrafficSplitting bool `json:"supports_traffic_splitting"`
}

func DefaultCapabilities(t Type) Capabilities {
	switch t {
	case TypeGKE:
		return Capabilities{SupportsCanary: true, SupportsBlueGreen: true, SupportsSecretsCSI: true}
	case TypeCloudRun:
		return Capabilities{SupportsCanary: true, SupportsBlueGreen: true, SupportsTrafficSplitting: true}
	case TypeMinikube:
		return Capabilities{SupportsCanary: false, SupportsBlueGreen: false, SupportsSecretsCSI: false}
	}
	return Capabilities{}
}

type Runtime struct {
	ID                  string         `json:"id"`
	WorkspaceID         string         `json:"workspace_id"`
	TenantID            string         `json:"tenant_id"`
	Type                Type           `json:"type"`
	Mode                Mode           `json:"mode"`
	Visibility          Visibility     `json:"visibility"`
	Name                string         `json:"name"`
	Region              string         `json:"region,omitempty"`
	GKEMode             GKEMode        `json:"gke_mode,omitempty"`
	ProjectID           string         `json:"project_id,omitempty"`
	ClusterName         string         `json:"cluster_name,omitempty"`
	Endpoint            string         `json:"endpoint,omitempty"`
	ServiceAccountEmail string         `json:"service_account_email,omitempty"`
	Namespace           string         `json:"namespace,omitempty"`
	CredentialKMSKeyRef string         `json:"credential_kms_key_ref,omitempty"`
	CredentialCipherB64 string         `json:"credential_cipher_b64,omitempty"`
	Labels              map[string]any `json:"labels,omitempty"`
	Capabilities        Capabilities   `json:"capabilities"`
	Status              string         `json:"status"`
	Revoked             bool           `json:"revoked"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}
