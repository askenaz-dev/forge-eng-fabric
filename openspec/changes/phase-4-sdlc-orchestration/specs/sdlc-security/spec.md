# Spec Delta: sdlc-security (ADDED)

## ADDED Requirements

### Requirement: Security skills

The capability SHALL expose `triage-vuln`, `propose-fix-for-finding`, `update-threat-model` as registered skills.

#### Scenario: Vulnerability triaged with proposed fix

- **GIVEN** a SAST finding `severity=high` on a Forge-managed repo
- **WHEN** Alfred invokes `triage-vuln`
- **THEN** the output MUST classify exploitability, propose remediation, link to CWE/CVE
- **AND** if remediation is straightforward, `propose-fix-for-finding` MUST open a PR with the fix and tests

### Requirement: Security gates

Gates `sast_clean`, `sca_clean`, `secrets_clean`, `dast_passed` (for `criticality≥high`) MUST be evaluated before progression to `devops`.

#### Scenario: Block progression on high-severity SAST finding

- **GIVEN** an initiative with an open `severity=high` SAST finding
- **WHEN** progression is requested
- **THEN** gate `sast_clean` MUST fail
- **AND** emit `sdlc.phase.blocked.v1` listing the findings
