# Skill gateway — end-to-end demo script

Use this script for the recorded demo that captures task 12.2. Requires a running platform stack (`make up`) plus a developer laptop with Claude Code installed.

## 0. One-time setup

```bash
# Bring the stack up
make up

# Seed the registry with a tenant + workspace
bash deploy/compose/scripts/seed-portal.sh
```

## 1. Author and package a skill

```bash
# We use the bundled reference skill as a stand-in
make package-skill \
  SPEC=reference-skills/generate-test-cases/skill.json \
  OUT=out/generate-test-cases@1.0.0.tar.zst

cat out/generate-test-cases@1.0.0.tar.zst.digest
```

## 2. Register the skill in the platform

```bash
# Create the asset in proposed state
curl -sS -X POST \
  -H "Authorization: Bearer $REGISTRY_TOKEN" \
  -H "content-type: application/json" \
  http://localhost:8082/v1/workspaces/$WS/assets \
  -d '{
    "type":"skill",
    "name":"generate-test-cases",
    "version":"1.0.0",
    "owner_team":"platform-engineering",
    "inputs_schema":{"type":"object"},
    "outputs_schema":{"type":"object"},
    "trust_level":"T2"
  }'

# Drive lifecycle to approved (pipeline-green + workspace-owner-approval omitted
# for brevity — the demo flow goes through both)
```

## 3. Publish to the gateway

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $REGISTRY_TOKEN" \
  -H "content-type: application/json" \
  http://localhost:8082/v1/assets/skill:$WS:generate-test-cases/versions/1.0.0/lifecycle-hooks/gateway-publish \
  -d '{
    "channel":"stable",
    "package_digest":"'$(cat out/generate-test-cases@1.0.0.tar.zst.digest)'",
    "signature_id":"demo-sig-1",
    "attestation_id":"demo-att-1",
    "bytes_uri":"s3://forge-packages/demo/generate-test-cases-1.0.0.tar.zst",
    "size_bytes":'$(wc -c < out/generate-test-cases@1.0.0.tar.zst)'
  }'
```

## 4. Install from a developer laptop

```bash
forge login --gateway http://localhost:8120
forge skills list
forge skills install generate-test-cases
ls ~/.claude/skills/generate-test-cases/
```

## 5. Invoke a tool via the MCP proxy

```bash
# Through Claude Code, ask the agent something that triggers the skill.
# Behind the scenes Claude Code dispatches MCP calls to:
#   http://localhost:8120/v1/gateway/mcp/<openspec-mcp-asset-id>
# with X-Forge-Principal set by the gateway.
```

## 6. Verify telemetry rolls up

Open `http://localhost:3000/gateway?workspace_id=$WS` in the portal. The just-installed asset shows:

- `gateway_published: true`
- the package digest
- `installs.active = 1`
- `by_source.gateway` populated under the asset's observability metrics
