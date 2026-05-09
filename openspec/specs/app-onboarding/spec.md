# app-onboarding Specification

## Purpose
TBD - created by archiving change bootstrap-forge-platform. Update Purpose after archive.
## Requirements
### Requirement: GitHub-first SCM integration
The platform SHALL integrate with **GitHub** as the initial SCM via authenticated GitHub Apps and SHALL support: connecting existing repositories to a Workspace, creating new repositories, configuring branch protection, opening pull requests, requesting reviews, and reacting to webhooks.

#### Scenario: Connect an existing GitHub repository to a Workspace
- **WHEN** an authorized user authenticates with GitHub and connects an existing repository to a Workspace
- **THEN** the repo is associated to the Workspace, webhooks are configured, and an audit event is emitted

#### Scenario: Alfred creates a new GitHub repository
- **WHEN** Alfred creates a new repository for a Workspace under autonomous policy
- **THEN** the repo is created with the configured org, name, visibility, branch protection and CODEOWNERS, and the repo is linked to the Workspace

### Requirement: Repository templates and scaffolding
The platform SHALL provide repository templates and scaffolding for supported app types (e.g., service, web, worker). Scaffolding SHALL include language-appropriate skeleton, lint, tests, CI configuration, Dockerfile, README and OpenSpec link.

#### Scenario: Scaffold a new service from template
- **WHEN** a user requests a new service from a template
- **THEN** Alfred generates the scaffold, opens an initial PR linked to the OpenSpec, and registers the repo with the Workspace

### Requirement: Branch policies and PR workflow
The platform SHALL configure branch policies (protected default branch, required status checks, required reviewers, CODEOWNERS) and SHALL produce PRs with: linked OpenSpec, summary, test evidence and quality/security gate results.

#### Scenario: PR includes OpenSpec link and gate results
- **WHEN** Alfred opens a PR
- **THEN** the PR description contains the OpenSpec link and is decorated with the latest SAST/SCA/quality results

### Requirement: Owners and CODEOWNERS
Each onboarded app SHALL have explicit technical, functional and operational owners reflected in repository CODEOWNERS and Workspace metadata.

#### Scenario: Cannot onboard app without owners
- **WHEN** an onboarding request omits required owners
- **THEN** the platform rejects the request with a clear validation error

### Requirement: Initial pipelines wired to the app
The platform SHALL configure initial CI/CD pipelines (using GitHub Actions and/or Cloud Build) for build, test, SAST/SCA, image build with SBOM and signing, and deploy hooks compatible with the Deployment Platform.

#### Scenario: Initial pipeline runs on first PR
- **WHEN** the first PR is opened on a newly onboarded repo
- **THEN** the CI pipeline runs lint, tests, SAST and SCA, and posts results on the PR

### Requirement: Future SCM integrations marked as out-of-scope for bootstrap
GitLab integration SHALL be considered a future capability; bootstrap SHALL NOT implement it.

#### Scenario: GitLab integration is not available in bootstrap
- **WHEN** a user requests GitLab integration during the bootstrap phase
- **THEN** the platform clearly indicates GitLab is on the roadmap and not available, without breaking GitHub flows

