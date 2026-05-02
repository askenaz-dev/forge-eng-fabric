# Spec Delta: mcp-and-skills (MODIFIED)

## MODIFIED Requirements

### Requirement: Editor consumes Registry catalog in real time

The visual editor and DSL parser MUST resolve node references against the Registry; references to non-existent or non-approved assets MUST be rejected at validation time.

#### Scenario: Reject reference to unknown skill

- **GIVEN** a DSL referencing `registry:skill/non-existent@1.0.0`
- **WHEN** validation runs
- **THEN** the parser MUST refuse with `400 unknown_asset`

### Requirement: Pinned references

Workflow steps MUST reference assets by exact id+version; floating tags (e.g., `latest`) MUST be rejected.

#### Scenario: Reject floating reference

- **GIVEN** a DSL referencing `registry:skill/refine-user-story@latest`
- **WHEN** validation runs
- **THEN** the parser MUST refuse with `400 floating_reference_not_allowed`
