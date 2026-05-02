# Spec Delta: workflow-visual-editor (ADDED)

## ADDED Requirements

### Requirement: Editor produces canonical AST

The visual editor MUST persist workflows as the canonical AST consumed by the runtime; the editor MUST NOT serialize a proprietary editor-only format.

#### Scenario: Save from editor matches DSL

- **GIVEN** a workflow built in the editor with 5 nodes and 1 branch
- **WHEN** saved
- **THEN** the persisted artifact MUST be the canonical AST
- **AND** exporting to DSL YAML MUST produce a valid file that re-imports identically

### Requirement: Live Registry catalog and validation

The editor MUST query the Registry for available skills/MCPs/prompts in real time and reject nodes referencing non-existent or non-approved assets.

#### Scenario: Reject node referencing in-review skill

- **GIVEN** a skill with `lifecycle_state=in_review`
- **WHEN** the user attempts to add it as a node
- **THEN** the editor MUST mark the node as invalid with reason `skill_not_approved`
- **AND** publish MUST be blocked

### Requirement: Debug dry-run

The editor MUST support a `dry_run` mode that executes the workflow with mocked I/O and surfaces the input/output of each step without invoking real tools.

#### Scenario: Dry-run does not call real MCPs

- **GIVEN** a workflow with `github.create_pr` step
- **WHEN** dry-run is executed
- **THEN** no actual GitHub call MUST occur
- **AND** the user MUST see mocked inputs/outputs based on declared schemas
