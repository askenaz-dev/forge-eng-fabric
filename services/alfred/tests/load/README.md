# Dedup Retrieval Load Test

Tests `POST /v1/intent/match` latency against the SLO target of **p95 < 100ms** (alfred-console-redesign §9.3, task 4.6).

## Quick start

```sh
# Ensure Alfred is running (local docker-compose or native)
make up

# 30-second warmup run, 10 virtual users
uv run python services/alfred/tests/load/dedup_load_test.py \
    --url http://localhost:8090 \
    --duration 30 \
    --vus 10

# Pilot-corpus validation (fail CI if SLO breached)
uv run python services/alfred/tests/load/dedup_load_test.py \
    --duration 90 \
    --vus 50 \
    --fail-on-slo-breach
```

## Output

```
=== Dedup Retrieval Load Test Results ===
Total requests : 1247
Successful     : 1247
Errors         : 0
Error rate     : 0.0%

Latency (ms):
  avg           42.3
  p50           38.1
  p90           71.4
  p95           84.2  ✓ SLO met
  p99           118.7
```

## Infrastructure requirements

The test sends real HTTP requests to Alfred, which in turn queries Milvus. Both services must be running:

| Service | Docker profile | Local port |
|---|---|---|
| Alfred | `base` | 8090 |
| Milvus | `rag` | 19530 |

```sh
COMPOSE_PROFILES=base,rag make up
```

The workspace must exist in Alfred's database (or dev-mode auth must be enabled via `DEV_AUTH_BYPASS=true`). Pass `--workspace-id <uuid>` if you have a pre-seeded workspace.

## CI integration

Add to the pilot-rollout validation job in `.github/workflows/alfred-load.yml`:

```yaml
- name: Dedup retrieval SLO
  run: |
    uv run python services/alfred/tests/load/dedup_load_test.py \
      --url $ALFRED_URL \
      --workspace-id $WS_ID \
      --token $ALFRED_TOKEN \
      --duration 90 \
      --vus 50 \
      --fail-on-slo-breach
```
