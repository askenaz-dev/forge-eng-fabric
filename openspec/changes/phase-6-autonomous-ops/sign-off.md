# Phase 6 — Autonomous Ops sign-off

This document captures the validation steps and stakeholder sign-off required
before the change is archived.

## Validation evidence

| Check                                                                        | Source                                                                                              | Status |
| ---------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- | ------ |
| Synthetic incidents covering L1..L5                                          | `python scripts/phase6_synthetic_incidents.py` — 5/5 incidents flow detect→diagnose→postmortem→KB   | ✅      |
| Diagnosis cites verifiable evidence                                          | `services/diagnosis/tests/test_pipeline.py::test_citation_enforcement_drops_unsupported_hypotheses` | ✅      |
| Healing executes correctly per level (incl. envelope cap, kill switch)       | `services/healing-engine/internal/engine/service_test.go` — all paths covered                       | ✅      |
| L5 auto-rollback                                                             | `services/healing-engine/internal/engine/service_test.go::TestL5AutoRollback`                       | ✅      |
| Postmortem auto-generated and published                                      | `services/postmortem/tests/test_generator.py::test_publish_calls_all_external_systems`              | ✅      |
| Postmortem eval rejects missing citations / owners                           | `services/postmortem/tests/test_generator.py::test_eval_suite_flags_missing_citation`               | ✅      |
| Evolution proposal created from postmortem                                   | `services/evolution/internal/evolution/service_test.go::TestFromPostmortemEmitsEvent`               | ✅      |
| Evolution acceptance ratio surfaced                                          | `services/openspec/tests/test_app.py::test_autonomous_loop_marker_and_review`                       | ✅      |
| Kill switch suppresses execution under active incident                       | `services/healing-engine/internal/engine/service_test.go::TestKillSwitchDegradesToL1`               | ✅      |
| Promotion of action L3→L4 in `dev` rejected without prerequisites            | `services/healing-engine/internal/engine/service_test.go::TestPromotionPrerequisitesUnmet`          | ✅      |
| Promotion of action L3→L4 succeeds when prerequisites are met                | `services/healing-engine/internal/engine/service_test.go::TestPromotionGrantsLevel`                 | ✅      |

## Documentation index

- [docs/healing/levels.md](../../../docs/healing/levels.md)
- [docs/healing/envelopes.md](../../../docs/healing/envelopes.md)
- [docs/postmortems/README.md](../../../docs/postmortems/README.md)
- [docs/evolution-loop/README.md](../../../docs/evolution-loop/README.md)
- [docs/finops/recommendations.md](../../../docs/finops/recommendations.md)

## Stakeholder sign-off

| Stakeholder        | Responsibility                                      | Signed |
| ------------------ | --------------------------------------------------- | ------ |
| Platform team      | Healing engine, observability, kill switch          | ⬜      |
| Security team      | Approval gates, kill switch, action promotion D6.10 | ⬜      |
| Tenant admin       | Per-tenant envelopes, audit trails                  | ⬜      |
| Workspace pilot #1 | First Workspace-scoped envelopes                    | ⬜      |
| Workspace pilot #2 | First Workspace-scoped envelopes                    | ⬜      |

Each stakeholder records their decision in this table at archive time. The
archive step will fail if any cell is left blank.
