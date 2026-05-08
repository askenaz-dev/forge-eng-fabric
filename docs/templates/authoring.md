# Template Authoring

Repo templates live under `forge-templates/templates/<template-id>/<semver>/template.yaml`.

## Manifest

Each `template.yaml` must define:

- `id`
- `version`
- `description`
- `category`
- `parameters` with type, required flag, defaults, regex patterns and enums
- `pre_hooks`
- `post_hooks`
- `files`
- `required_capabilities`

Example parameter:

```yaml
parameters:
  name:
    type: string
    required: true
    pattern: "^[a-z][a-z0-9-]{1,40}$"
```

## Governance

Production onboarding only uses template versions that are registered as Registry assets with:

- `type=repo_template`
- `lifecycle_state=approved`
- `trust_level>=T3`
- immutable SemVer version
- signed release tag

Corrections require publishing a new SemVer version. Published versions must not be overwritten.

## Hooks

Hooks run inside the render output directory. Keep hooks deterministic, short-lived and free of external secrets. External network access should be avoided unless a policy explicitly allows it.
