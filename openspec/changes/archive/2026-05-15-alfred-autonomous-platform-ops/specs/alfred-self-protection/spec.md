## ADDED Requirements

### Requirement: Hard denylist for self and critical dependencies

A Rego policy `policies/alfred/self-protection.rego` SHALL evaluate before `risk-classifier.rego` in the policy chain. Any action whose `target` resolves to one of `alfred`, `symptom-triager`, `platform-ops`, `opa`, `keycloak` SHALL be denied irrespective of any other input or override.

#### Scenario: Restart of platform-ops is denied

- **WHEN** any caller (including Alfred) invokes a mutating action whose target service is `platform-ops`
- **THEN** OPA SHALL return `autonomy_decision:"deny"` with `reason:"self-protection: target on denylist"`
- **AND** the endpoint SHALL return 403 and record an audit row with `outcome:"denied_by_self_protection"`

#### Scenario: Restart of Keycloak is denied

- **WHEN** the action targets `keycloak`
- **THEN** the policy SHALL deny regardless of `actor`, `trigger_source`, blast radius, or any override

### Requirement: Override is mechanically impossible

The self-protection policy SHALL NOT honour any tenant or workspace override. The bundle build SHALL reject any override that attempts to relax the denylist with a build-time failure `code=self_protection_override_attempted`.

#### Scenario: Override attempt fails bundle build

- **WHEN** a tenant override file lists `platform-ops` as `autonomous`
- **THEN** the OPA bundle pipeline SHALL fail with `code=self_protection_override_attempted` and SHALL NOT publish the bundle

### Requirement: Outages of denylisted targets page humans

When a symptom is detected whose service is on the self-protection denylist, the triager SHALL NOT spawn an autonomous session. It SHALL instead enqueue a critical-severity HITL ticket and page on-call directly through the existing notification path.

#### Scenario: platform-ops down → human page

- **WHEN** `symptom-emitter-probe` reports `service:platform-ops|signal:probe-failed`
- **THEN** the triager SHALL create a `severity:critical` HITL ticket and trigger the page channel (PagerDuty or equivalent)
- **AND** SHALL NOT spawn any autonomous session

### Requirement: Audit signature of attempted bypasses

Any rejected action against the denylist SHALL produce a tamper-evident audit row that is escalated to the security audit channel separately from regular audit.

#### Scenario: Bypass attempt is escalated

- **WHEN** OPA denies an action via self-protection
- **THEN** the audit row SHALL be tagged `audit_channel="security"` and a duplicate event SHALL be emitted to `forge.security.audit.v1` for high-priority review
