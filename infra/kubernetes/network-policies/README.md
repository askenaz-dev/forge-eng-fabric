# Forge Network Policies

These manifests enforce the Phase 0 model gateway boundary in Kubernetes:

- Workload namespaces default-deny egress.
- Forge workloads may call DNS, internal Forge services and `litellm` only.
- Only the `litellm` namespace is allowed to reach approved external model providers.

Kubernetes `NetworkPolicy` cannot match external hostnames. The Cilium policy adds the provider FQDN allowlist for clusters that support Cilium FQDN policies; on GKE Dataplane V2 this should be paired with Cloud Firewall / Cloud DNS policy from `docs/policies/network-egress.md`.
