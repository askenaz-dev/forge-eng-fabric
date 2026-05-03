App Onboarding Service (stub)

This is a minimal stub of the app-onboarding service. It exposes a small HTTP endpoint POST /v1/onboarding that validates input minimally and returns a request id. The full implementation would orchestrate policy evaluation, scaffolder invocation, MCP calls, branch protections, pipeline publication and asset registration.
