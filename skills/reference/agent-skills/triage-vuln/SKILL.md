---
name: triage-vuln
description: Triage a vulnerability finding. Use when SAST, SCA, DAST, or manual security findings need exploitability and remediation context.
license: Apache-2.0
compatibility: Forge Alfred skill runner; Python package reference_skills
metadata:
  forge.asset_type: skill
  forge.asset_id: skill-triage-vuln
  forge.tool_id: skill:triage-vuln
  forge.runtime: reference_skills.skills:triage_vuln
  forge.eval_suite: deterministic-reference-skills
---

# triage-vuln

Use this SDLC security skill to produce a vulnerability triage artifact.

Return JSON with `skill`, `capability`, `artifacts`, `recommendations`, `links`, and `eval_score`.
