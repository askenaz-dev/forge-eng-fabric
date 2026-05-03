<#
PowerShell helper to run the Phase 1 integrated smoke checks.

Usage (PowerShell):
  $env:ALFRED_API_URL = "https://alfred.staging.example"
  $env:REGISTRY_API_URL = "https://registry.staging.example"
  $env:APPROVALS_API_URL = "https://approvals.staging.example"
  $env:LANGFUSE_API_KEY = "<redacted>"
  ./scripts/integration/run_phase1_integrated_checks.ps1

This script is a template. Replace variables with real endpoints and secure secrets where appropriate.
#>

function AbortIfEmpty([string]$val, [string]$name) {
    if (-not $val) {
        Write-Error "$name is not set. Export as environment variable before running."
        exit 2
    }
}

AbortIfEmpty $env:ALFRED_API_URL "ALFRED_API_URL"
AbortIfEmpty $env:REGISTRY_API_URL "REGISTRY_API_URL"
AbortIfEmpty $env:APPROVALS_API_URL "APPROVALS_API_URL"

Write-Host "ALFRED_API_URL = $env:ALFRED_API_URL"

function HealthCheck($url) {
    try {
        $resp = Invoke-RestMethod -Uri "$url/health" -Method Get -ErrorAction Stop
        Write-Host "Health OK: $url"
    } catch {
        Write-Error "Health check failed for $url: $_"
        exit 3
    }
}

HealthCheck $env:ALFRED_API_URL
HealthCheck $env:REGISTRY_API_URL
HealthCheck $env:APPROVALS_API_URL

# 1. Create a test workspace via Registry (or assume an existing one)
$workspaceId = [guid]::NewGuid().ToString()
Write-Host "Using workspace: $workspaceId"

# 2. Grant Alfred delegated permission (via Approvals/Permissions API)
$grantPayload = @{
    subject = "alfred"
    action_class = "openspec:write"
    scope_kind = "workspace"
    scope_id = $workspaceId
    max_criticality = "medium"
    expiration_days = 7
} | ConvertTo-Json

Write-Host "Granting delegated permission to Alfred (simulation)"
try {
    $grantResp = Invoke-RestMethod -Uri "$env:APPROVALS_API_URL/v1/grants" -Method Post -Body $grantPayload -ContentType 'application/json' -ErrorAction Stop
    Write-Host "Grant response: $(($grantResp | ConvertTo-Json -Depth 5))"
} catch {
    Write-Warning "Grant endpoint failed or not implemented; continue if environment uses different mechanism: $_"
}

# 3. Submit Alfred intent
$correlationId = "phase-1-integrated-$(Get-Random)"
$intentPayload = @{
    actor = "alice"
    workspace_id = $workspaceId
    intent = "Validate Phase 1 integrated E2E"
    correlation_id = $correlationId
    openspec_id = "phase-1-integrated"
    metadata = @{ env = "dev" }
} | ConvertTo-Json

Write-Host "Submitting intent with correlation_id: $correlationId"
try {
    $intentResp = Invoke-RestMethod -Uri "$env:ALFRED_API_URL/v1/intents" -Method Post -Body $intentPayload -ContentType 'application/json' -ErrorAction Stop
    Write-Host "Intent submitted: $($intentResp | ConvertTo-Json -Depth 5)"
} catch {
    Write-Error "Failed to submit intent: $_"
    exit 4
}

# 4. Poll decisions endpoint until final decision or timeout
$decisionsUrl = "$env:ALFRED_API_URL/v1/decisions?correlation_id=$correlationId"
$deadline = (Get-Date).AddMinutes(5)
while ((Get-Date) -lt $deadline) {
    try {
        $decisions = Invoke-RestMethod -Uri $decisionsUrl -Method Get -ErrorAction Stop
        if ($decisions -and $decisions.Count -gt 0) {
            Write-Host "Decisions found: $($decisions | ConvertTo-Json -Depth 5)"
            break
        }
    } catch {
        Write-Warning "Polling decisions failed: $_"
    }
    Start-Sleep -Seconds 3
}

if (-not $decisions) {
    Write-Error "No decisions found for correlation_id: $correlationId"
    exit 5
}

Write-Host "Basic verification done. Collect Langfuse and Tempo traces for correlation id: $correlationId"
Write-Host "Follow the runbook in docs/governance/phase-1-integrated-runbook.md to continue manual/instrumented verification."

exit 0
