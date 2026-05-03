<#
PowerShell helper for Phase 1 integrated evidence checks.

Required for registry checks:
  $env:REGISTRY_API_URL = "https://registry.staging.example"
  $env:WORKSPACE_ID = "<workspace-uuid>"
  $env:AUTH_TOKEN = "<bearer-token>"

Additional for Alfred/Langfuse full E2E checks:
  $env:ALFRED_API_URL = "https://alfred.staging.example"
  $env:ALFRED_TOKEN = "<bearer-token>" # optional; AUTH_TOKEN is used as fallback
  $env:LANGFUSE_API_URL = "https://langfuse.staging.example"
  $env:LANGFUSE_PUBLIC_KEY = "<public-key>"
  $env:LANGFUSE_SECRET_KEY = "<secret-key>"

Evidence is written by pytest to docs/governance/evidence/phase-1/<timestamp>/.
#>

$ErrorActionPreference = "Stop"

function AbortIfEmpty([string]$Value, [string]$Name) {
    if (-not $Value) {
        Write-Error "$Name is not set. Export it as an environment variable before running."
        exit 2
    }
}

function HealthCheck([string]$Url, [string]$Name) {
    if (-not $Url) {
        return
    }

    foreach ($Path in @("/healthz", "/readyz")) {
        try {
            Invoke-WebRequest -Uri "$($Url.TrimEnd('/'))$Path" -Method Get -UseBasicParsing | Out-Null
            Write-Host "$Name $Path OK"
            return
        } catch {
            Write-Verbose "$Name $Path failed: $_"
        }
    }

    Write-Error "$Name health check failed for $Url"
    exit 3
}

AbortIfEmpty $env:REGISTRY_API_URL "REGISTRY_API_URL"
AbortIfEmpty $env:WORKSPACE_ID "WORKSPACE_ID"
AbortIfEmpty $env:AUTH_TOKEN "AUTH_TOKEN"

HealthCheck $env:REGISTRY_API_URL "Registry"
HealthCheck $env:ALFRED_API_URL "Alfred"

Write-Host "Running Phase 1 integrated pytest checks..."
uv run pytest -q services/registry/tests/test_integration_promotion.py
$ExitCode = $LASTEXITCODE

if ($ExitCode -ne 0) {
    Write-Error "Phase 1 integrated checks failed with exit code $ExitCode"
    exit $ExitCode
}

Write-Host "Phase 1 integrated checks completed. Review evidence under docs/governance/evidence/phase-1/."
exit 0
