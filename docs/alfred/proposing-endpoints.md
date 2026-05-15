# Proposing a New Platform-Ops Endpoint

This document describes the **official escape hatch** for extending Alfred's
autonomous action surface when a required action does not map to any existing
`platform-ops` endpoint.

Alfred operates exclusively through semantic, auditable endpoints — never via
generic shell access.  Every endpoint exists because a human engineer reviewed,
scoped, and approved the blast radius.  The process below maintains that
invariant while allowing the platform to grow.

---

## When to use this process

Use this process when:

- Alfred (or a playbook author) needs to perform an action that is currently
  not modelled in `services/platform-ops`.
- The action is repeatable, auditable, and can be expressed as a typed
  input/output schema.
- The action is **not** achievable by composing existing endpoints.

Do **not** use this process to:

- Work around an OPA deny decision — update the policy instead.
- Bypass dual-approval requirements — adjust the approval flow instead.
- Expose a generic exec/shell surface.

---

## Step-by-step process

### 1. Open a Platform-Ops Proposal PR

Create a PR in this repository with the following artefacts:

```
services/platform-ops/internal/server/<action>.go   # handler
contracts/openapi/platform-ops.yaml                  # OpenAPI schema update
policies/alfred/risk-classifier.rego                 # OPA classification
db/migrations/platform-ops/XXXX_<action>.sql         # if schema changes needed
```

PR title format: `feat(platform-ops): add <action> endpoint`

The PR description must include:

| Field | Required content |
|---|---|
| **Action class** | One of: `diagnose`, `mutate-runtime`, `mutate-data`, `mutate-config`, `mutate-code`, `mutate-infra` |
| **Blast radius** | `service`, `workspace`, `tenant`, or `repository` |
| **Reversibility** | `trivial`, `easy`, `hard`, or `irreversible` |
| **Expected OPA outcome** | `allow`, `requires_approval`, or `requires_dual_control` |
| **Post-validate probe** | How the endpoint verifies the action succeeded |
| **Rollback action** | Which existing endpoint (or `none`) reverts this action |

### 2. OPA policy classification

Add a rule to `policies/alfred/risk-classifier.rego` classifying the new action:

```rego
# Example: a new "drain node" mutate-infra action
autonomy_decision := "requires_approval" if {
    input.action_class == "mutate-infra"
    input.action == "drain_node"
}
```

Run `opa check policies/alfred --strict` and `opa test policies/alfred -v` locally
before submitting.  CI will fail on Rego compile errors.

### 3. Self-protection check

If the endpoint could target a protected service (Alfred itself, OPA, Keycloak,
OpenFGA, platform-ops, or the symptom-triager), add an explicit guard at the top
of the handler:

```go
denied, err := h.cfg.OPA.EvalSelfProtection(r.Context(), target)
if err != nil || denied {
    writeError(w, http.StatusForbidden, "self-protection: target is protected")
    return
}
```

### 4. Audit row with bundle hash

Every mutating handler must write an audit row that includes the
`PolicyBundleHash` field:

```go
h.cfg.Audit.Write(ctx, audit.Row{
    Actor:            actor,
    Action:           "your_action",
    Target:           target,
    Outcome:          "success",
    PolicyBundleHash: h.cfg.OPA.BundleHash(),
    // ...
})
```

A missing or mismatched hash fires the `AlfredBundleHashDangling` alert.

### 5. Dual approval for irreversible actions

If `reversibility = irreversible` or `action_class = mutate-infra`, the endpoint
**must** implement dual-approval flow:

```go
decision, err := h.cfg.OPA.EvalRiskClassifierFull(ctx, input)
if decision.ApprovalMode == "dual" && !isAutonomous {
    writeJSON(w, http.StatusAccepted, map[string]any{
        "status":    "pending_approval",
        "approvers": decision.Approvers,
    })
    return
}
```

### 6. Post-validate probe

After the mutating action, call the relevant probe to confirm the desired state:

```go
ok, err := h.cfg.Probe.Check(ctx, probe.HTTPRequest{
    URL:            fmt.Sprintf("http://%s/healthz", target),
    ExpectedStatus: 200,
})
if !ok {
    // auto-rollback if reversibility is trivial or easy
}
```

### 7. Review checklist before merging

- [ ] OPA unit tests cover the new action class
- [ ] `lattice-check.sh` passes (no tenant override relaxes the global rule)
- [ ] Audit row includes `PolicyBundleHash`
- [ ] Post-validate probe is implemented
- [ ] Rollback endpoint exists or is explicitly documented as `none`
- [ ] OpenAPI schema is updated and passes `redocly lint`
- [ ] Self-protection check added if the target could be a protected service

---

## Approval

New endpoints require sign-off from **two of**: platform team lead, security
reviewer, and on-call engineer.  This matches the dual-approval requirement for
any change to Alfred's autonomous action surface.

Once merged, the endpoint is available to Alfred in the next bundle build cycle.
