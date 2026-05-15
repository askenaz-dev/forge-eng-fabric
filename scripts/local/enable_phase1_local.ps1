<#
Enable a local Phase 1 integrated slice.

This script uses local development credentials only:
  Keycloak user: alice
  Keycloak password: alice
  Keycloak client: forge-cli

It starts compose dependencies, applies local migrations, seeds a deterministic
workspace, starts Phase 1 services from source, and runs the Registry evidence
test for tasks 13.5 and 13.6.

Usage:
  ./scripts/local/enable_phase1_local.ps1
  ./scripts/local/enable_phase1_local.ps1 -SkipEvidence
  ./scripts/local/enable_phase1_local.ps1 -Stop
#>

param(
    [switch]$SkipEvidence,
    [switch]$Stop
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$ComposeFile = Join-Path $Root "deploy\compose\docker-compose.yaml"
$DataDir = Join-Path $Root "deploy\compose\data\phase1-local"
$PidFile = Join-Path $DataDir "pids.json"
$EnvFile = Join-Path $DataDir "env.ps1"
$WorkspaceId = "33333333-3333-4333-8333-333333333333"
$TenantId = "11111111-1111-4111-8111-111111111111"
$BusinessUnitId = "22222222-2222-4222-8222-222222222222"

function ComposeArgs() {
    return @("compose", "-f", $ComposeFile)
}

function Invoke-DockerCompose {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$ComposeCommandArgs)
    & docker @((ComposeArgs) + $ComposeCommandArgs)
}

function Stop-LocalProcesses() {
    if (-not (Test-Path $PidFile)) {
        Write-Host "No Phase 1 local PID file found."
        return
    }
    $pids = Get-Content -Raw $PidFile | ConvertFrom-Json
    foreach ($entry in $pids) {
        try {
            $process = Get-Process -Id $entry.pid -ErrorAction Stop
            Stop-Process -Id $process.Id -Force
            Write-Host "Stopped $($entry.name) pid=$($entry.pid)"
        } catch {
            Write-Host "Already stopped $($entry.name) pid=$($entry.pid)"
        }
    }
    Remove-Item $PidFile -Force
}

if ($Stop) {
    Stop-LocalProcesses
    exit 0
}

New-Item -ItemType Directory -Force -Path $DataDir | Out-Null

function Wait-Http([string]$Url, [int]$Seconds = 120) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        try {
            Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 5 | Out-Null
            Write-Host "OK $Url"
            return
        } catch {
            Start-Sleep -Seconds 2
        }
    }
    throw "Timed out waiting for $Url"
}

function Wait-Postgres([int]$Seconds = 120) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        Invoke-DockerCompose @("exec", "-T", "postgres", "pg_isready", "-U", "forge") *> $null
        if ($LASTEXITCODE -eq 0) {
            Write-Host "OK postgres"
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "Timed out waiting for postgres"
}

function Invoke-Psql([string]$Database, [string]$Sql) {
    $Sql | & docker @((ComposeArgs) + @("exec", "-T", "postgres", "psql", "-v", "ON_ERROR_STOP=1", "-U", "forge", "-d", $Database))
    if ($LASTEXITCODE -ne 0) {
        throw "psql failed for database $Database"
    }
}

function Query-Psql([string]$Database, [string]$Sql) {
    $out = & docker @((ComposeArgs) + @("exec", "-T", "postgres", "psql", "-At", "-U", "forge", "-d", $Database, "-c", $Sql))
    if ($LASTEXITCODE -ne 0) {
        throw "psql query failed for database $Database"
    }
    return ($out | Where-Object { $_ -ne $null } | ForEach-Object { $_.Trim() })
}

function Ensure-Database([string]$Name) {
    $exists = Query-Psql "postgres" "SELECT 1 FROM pg_database WHERE datname = '$Name';"
    if ($exists -notcontains "1") {
        Invoke-Psql "postgres" "CREATE DATABASE $Name OWNER forge;"
        Write-Host "Created database $Name"
    }
}

function Ensure-MigrationTable([string]$Database) {
    Invoke-Psql $Database "CREATE TABLE IF NOT EXISTS forge_local_migrations(version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());"
}

function Get-UpSql([string]$Path) {
    $raw = Get-Content -Raw -Path $Path
    return ($raw -split "-- \+goose Down", 2)[0]
}

function Apply-Migration([string]$Database, [string]$Version, [string]$Path) {
    Ensure-MigrationTable $Database
    $applied = Query-Psql $Database "SELECT 1 FROM forge_local_migrations WHERE version = '$Version';"
    if ($applied -contains "1") {
        Write-Host "Migration already applied: $Database $Version"
        return
    }
    $sql = Get-UpSql $Path
    Invoke-Psql $Database $sql
    Invoke-Psql $Database "INSERT INTO forge_local_migrations(version) VALUES ('$Version');"
    Write-Host "Applied migration: $Database $Version"
}

function Seed-Workspace() {
    $sql = @"
INSERT INTO tenant(id, name, created_by)
VALUES ('$TenantId', 'local-phase1', 'alice')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

INSERT INTO business_unit(id, tenant_id, name, created_by)
VALUES ('$BusinessUnitId', '$TenantId', 'platform', 'alice')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

INSERT INTO workspace(id, tenant_id, business_unit_id, name, description, owners, created_by)
VALUES ('$WorkspaceId', '$TenantId', '$BusinessUnitId', 'phase1-local', 'Local Phase 1 validation workspace', ARRAY['alice'], 'alice')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, owners = EXCLUDED.owners;
"@
    Invoke-Psql "forge_control_plane" $sql
    Write-Host "Seeded workspace $WorkspaceId"
}

function Start-ManagedProcess([string]$Name, [string]$WorkingDirectory, [string]$Command, [string]$HealthUrl) {
    try {
        Invoke-WebRequest -Uri $HealthUrl -UseBasicParsing -TimeoutSec 2 | Out-Null
        Write-Host "$Name already healthy at $HealthUrl"
        return $null
    } catch {
        # Start below.
    }

    $serviceDir = Join-Path $DataDir $Name
    New-Item -ItemType Directory -Force -Path $serviceDir | Out-Null
    $scriptPath = Join-Path $serviceDir "run.ps1"
    Set-Content -Path $scriptPath -Encoding UTF8 -Value $Command
    $stdout = Join-Path $serviceDir "stdout.log"
    $stderr = Join-Path $serviceDir "stderr.log"
    $proc = Start-Process -FilePath "pwsh" -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $scriptPath) -WorkingDirectory $WorkingDirectory -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru
    Write-Host "Started $Name pid=$($proc.Id)"
    Wait-Http $HealthUrl 300
    return @{ name = $Name; pid = $proc.Id; health = $HealthUrl }
}

function Get-KeycloakToken() {
    $body = @{
        client_id = "forge-cli"
        username = "alice"
        password = "alice"
        grant_type = "password"
        scope = "openid profile email"
    }
    $resp = Invoke-RestMethod -Method Post -Uri "http://localhost:8080/realms/forge/protocol/openid-connect/token" -ContentType "application/x-www-form-urlencoded" -Body $body
    return $resp.access_token
}

Write-Host "Starting local compose dependencies..."
Invoke-DockerCompose @("up", "-d")
Wait-Postgres
Wait-Http "http://localhost:8080/realms/forge/.well-known/openid-configuration" 180

foreach ($db in @("forge_control_plane", "forge_registry", "forge_audit", "forge_alfred", "forge_openspec", "forge_approvals", "forge_permissions", "forge_prompt_registry")) {
    Ensure-Database $db
}

Apply-Migration "forge_control_plane" "control-plane/0001_init.sql" (Join-Path $Root "db\migrations\control-plane\0001_init.sql")
Apply-Migration "forge_control_plane" "control-plane/0002_platform_user.sql" (Join-Path $Root "db\migrations\control-plane\0002_platform_user.sql")
Apply-Migration "forge_registry" "registry/0001_init.sql" (Join-Path $Root "db\migrations\registry\0001_init.sql")
Apply-Migration "forge_registry" "registry/0002_phase1_lifecycle.sql" (Join-Path $Root "db\migrations\registry\0002_phase1_lifecycle.sql")
Apply-Migration "forge_audit" "audit/0001_init.sql" (Join-Path $Root "db\migrations\audit\0001_init.sql")
Apply-Migration "forge_alfred" "alfred/0001_init.sql" (Join-Path $Root "db\migrations\alfred\0001_init.sql")

Seed-Workspace

$processes = @()
$AlfredDefaultModel = if ($env:ALFRED_DEFAULT_MODEL) { $env:ALFRED_DEFAULT_MODEL } else { "gemini-1.5-pro" }
$commonGoEnv = @"
`$env:KEYCLOAK_ISSUER = 'http://localhost:8080/realms/forge'
`$env:KEYCLOAK_AUDIENCE = 'forge-control-plane'
`$env:OPENFGA_STORE_ID = ''
`$env:OPENFGA_AUTHORIZATION_MODEL_ID = ''
`$env:KAFKA_BROKERS = 'localhost:29092'
`$env:EVENTS_TOPIC = 'forge.events'
`$env:OTEL_EXPORTER_OTLP_ENDPOINT = 'localhost:4317'
"@

$controlPlaneCommand = @"
$commonGoEnv
`$env:ADDR = ':8081'
`$env:POSTGRES_URL = 'postgres://forge:forge@localhost:15432/forge_control_plane?sslmode=disable'
go run ./cmd/server
"@
$processes += Start-ManagedProcess "control-plane" (Join-Path $Root "services\control-plane") $controlPlaneCommand "http://localhost:8081/readyz"

$registryCommand = @"
$commonGoEnv
`$env:ADDR = ':8082'
`$env:POSTGRES_URL = 'postgres://forge:forge@localhost:15432/forge_registry?sslmode=disable'
`$env:CONTROL_PLANE_DB_URL = 'postgres://forge:forge@localhost:15432/forge_control_plane?sslmode=disable'
go run ./cmd/server
"@
$processes += Start-ManagedProcess "registry" (Join-Path $Root "services\registry") $registryCommand "http://localhost:8082/healthz"

$policyCommand = @"
`$env:ADDR = ':8084'
go run ./cmd/server
"@
$processes += Start-ManagedProcess "policy-engine" (Join-Path $Root "services\policy-engine") $policyCommand "http://localhost:8084/healthz"

$openspecCommand = @"
`$env:OPENSPEC_ROOT = '$((Join-Path $Root "openspec\records") -replace "'", "''")'
uv run uvicorn openspec_service.app:app --host 127.0.0.1 --port 8083
"@
$processes += Start-ManagedProcess "openspec" (Join-Path $Root "services\openspec") $openspecCommand "http://localhost:8083/healthz"

$approvalsCommand = @"
`$env:APPROVALS_STORE_PATH = '$((Join-Path $DataDir "approvals.json") -replace "'", "''")'
uv run uvicorn approvals.app:app --host 127.0.0.1 --port 8105
"@
$processes += Start-ManagedProcess "approvals" (Join-Path $Root "services\approvals") $approvalsCommand "http://localhost:8105/healthz"

$appOnboardingCommand = @"
`$env:ADDR = ':8085'
go run ./cmd
"@
$processes += Start-ManagedProcess "app-onboarding" (Join-Path $Root "services\app-onboarding") $appOnboardingCommand "http://localhost:8085/healthz"

$ragQueryCommand = @"
`$env:OPENFGA_STORE_ID = ''
`$env:OPENFGA_AUTHORIZATION_MODEL_ID = ''
uv run uvicorn rag_query.app:app --host 127.0.0.1 --port 8086
"@
$processes += Start-ManagedProcess "rag-query" (Join-Path $Root "services\rag-query") $ragQueryCommand "http://localhost:8086/healthz"

$promptRegistryCommand = @"
uv run uvicorn prompt_registry.app:app --host 127.0.0.1 --port 8087
"@
$processes += Start-ManagedProcess "prompt-registry" (Join-Path $Root "services\prompt-registry") $promptRegistryCommand "http://localhost:8087/healthz"

$skillRunnerCommand = @"
uv run uvicorn reference_skills.runner:app --host 127.0.0.1 --port 8091
"@
$processes += Start-ManagedProcess "skill-runner" (Join-Path $Root "skills\reference") $skillRunnerCommand "http://localhost:8091/healthz"

$permissionsCommand = @"
`$env:PERMISSIONS_STORE_PATH = '$((Join-Path $DataDir "permissions.json") -replace "'", "''")'
uv run uvicorn permissions.app:app --host 127.0.0.1 --port 8092
"@
$processes += Start-ManagedProcess "permissions" (Join-Path $Root "services\permissions") $permissionsCommand "http://localhost:8092/healthz"

$mcpOpenSpecCommand = @"
`$env:OPENSPEC_URL = 'http://localhost:8083'
uv run uvicorn servers.openspec:app --host 127.0.0.1 --port 8104
"@
$processes += Start-ManagedProcess "mcp-openspec" (Join-Path $Root "services\mcp") $mcpOpenSpecCommand "http://localhost:8104/healthz"

$alfredCommand = @"
`$env:ADDR = '127.0.0.1:8090'
`$env:KEYCLOAK_ISSUER = 'http://localhost:8080/realms/forge'
`$env:KEYCLOAK_AUDIENCE = 'forge-control-plane'
`$env:OPENFGA_STORE_ID = ''
`$env:OPENFGA_AUTHORIZATION_MODEL_ID = ''
`$env:POSTGRES_URL = 'postgres://forge:forge@localhost:15432/forge_alfred?sslmode=disable'
`$env:POLICY_ENGINE_URL = 'http://localhost:8084'
`$env:APPROVALS_URL = 'http://localhost:8105'
`$env:RAG_QUERY_URL = 'http://localhost:8086'
`$env:PROMPT_REGISTRY_URL = 'http://localhost:8087'
`$env:SKILL_RUNNER_URL = 'http://localhost:8091'
`$env:PERMISSIONS_URL = 'http://localhost:8092'
`$env:MCP_OPENSPEC_URL = 'http://localhost:8104'
`$env:ALFRED_DIALOGUE_API = 'enabled'
`$env:ALFRED_DEFAULT_MODEL = '$($AlfredDefaultModel -replace "'", "''")'
uv run uvicorn alfred.app:app --host 127.0.0.1 --port 8090
"@
$processes += Start-ManagedProcess "alfred" (Join-Path $Root "services\alfred") $alfredCommand "http://localhost:8090/readyz"

$processes = @($processes | Where-Object { $_ -ne $null })
if ($processes.Count -gt 0) {
    $processes | ConvertTo-Json -Depth 4 | Set-Content -Path $PidFile -Encoding UTF8
}

$token = Get-KeycloakToken
$envContent = @"
`$env:REGISTRY_API_URL = 'http://localhost:8082'
`$env:WORKSPACE_ID = '$WorkspaceId'
`$env:AUTH_TOKEN = '$token'
`$env:ALFRED_API_URL = 'http://localhost:8090'
`$env:ALFRED_TOKEN = '$token'
"@
Set-Content -Path $EnvFile -Encoding UTF8 -Value $envContent

Write-Host "Local Phase 1 environment:"
Write-Host "  REGISTRY_API_URL=http://localhost:8082"
Write-Host "  ALFRED_API_URL=http://localhost:8090"
Write-Host "  WORKSPACE_ID=$WorkspaceId"
Write-Host "  AUTH_TOKEN=<alice local Keycloak token>"
Write-Host "Environment file written to $EnvFile"

if (-not $SkipEvidence) {
    $env:REGISTRY_API_URL = "http://localhost:8082"
    $env:WORKSPACE_ID = $WorkspaceId
    $env:AUTH_TOKEN = $token
    Remove-Item Env:ALFRED_API_URL -ErrorAction SilentlyContinue
    Remove-Item Env:ALFRED_TOKEN -ErrorAction SilentlyContinue
    Remove-Item Env:LANGFUSE_API_URL -ErrorAction SilentlyContinue
    Remove-Item Env:LANGFUSE_HOST -ErrorAction SilentlyContinue
    Remove-Item Env:LANGFUSE_PUBLIC_KEY -ErrorAction SilentlyContinue
    Remove-Item Env:LANGFUSE_SECRET_KEY -ErrorAction SilentlyContinue
    Remove-Item Env:LANGFUSE_API_KEY -ErrorAction SilentlyContinue

    Write-Host "Running local Phase 1 Registry evidence test..."
    Push-Location $Root
    try {
        uv run pytest -q services/registry/tests/test_integration_promotion.py -k registry_promotion
        if ($LASTEXITCODE -ne 0) {
            throw "local Registry evidence test failed"
        }
    } finally {
        Pop-Location
    }
}

Write-Host "Phase 1 local enablement complete. Stop app processes with: ./scripts/local/enable_phase1_local.ps1 -Stop"
