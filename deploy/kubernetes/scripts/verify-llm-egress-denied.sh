#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-forge-system}"
POD="forge-llm-egress-negative"

kubectl apply -f infra/kubernetes/tests/llm-egress-negative.yaml
kubectl wait --for=condition=Ready=false "pod/${POD}" -n "${NAMESPACE}" --timeout=10s || true
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded "pod/${POD}" -n "${NAMESPACE}" --timeout=60s
kubectl logs "pod/${POD}" -n "${NAMESPACE}"
kubectl delete pod "${POD}" -n "${NAMESPACE}" --ignore-not-found=true
