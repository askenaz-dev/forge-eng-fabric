# Network Egress Policy Draft

Status: draft for Security/SRE validation.

Forge requires all model provider access to pass through LiteLLM. Phase 0 enforces this by convention in docker-compose; Kubernetes NetworkPolicies and cloud egress firewall rules are deferred to the cluster slice.

## Intended Rule

Only the LiteLLM runtime may initiate outbound connections to approved model provider endpoints. All other Forge services must call LiteLLM through the internal gateway endpoint.

## Local Endpoints

| Component | Endpoint | Notes |
|---|---|---|
| LiteLLM gateway | `http://localhost:4000` | Local host port |
| LiteLLM in compose network | `http://litellm:4000` | Service-to-service endpoint |
| Local key | `sk-forge-local` | Development-only master key |

## Approved Provider Hostnames

| Provider | Hostnames | Status |
|---|---|---|
| Vertex AI | `*.googleapis.com` | Planned for cloud bootstrap; credentials not configured locally |
| OpenAI-compatible stub | Local LiteLLM config only | Used for Phase 0 smoke tests |

## Kubernetes Enforcement Design

When Helm/Kubernetes deployment is introduced, apply:

1. Default deny egress for platform namespaces.
2. Allow Forge services to call only internal platform services, including `litellm`.
3. Allow only the LiteLLM namespace/pod selector to egress to approved provider hostnames or egress gateway destinations.
4. Log denied egress attempts as security events.
5. Add a negative test pod that tries to reach a provider endpoint from a non-LiteLLM namespace and must fail.

## Phase 0 Status

| Control | Status |
|---|---|
| Application calls route through LiteLLM | Implemented for local stub flows |
| Docker network isolation | Compose network exists; provider egress is not blocked at host level |
| Kubernetes NetworkPolicy | Deferred |
| Cloud firewall / egress gateway | Deferred |
| Negative egress test | Deferred until Kubernetes exists |

## Review Questions

1. Which providers are approved for the first cloud environment?
2. Will egress control be enforced by Kubernetes NetworkPolicy, service mesh, cloud firewall, or a combination?
3. Should DNS allow-lists be managed centrally by SRE?
4. What alert severity applies to direct provider egress attempts outside LiteLLM?
