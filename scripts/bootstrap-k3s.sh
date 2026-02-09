#!/usr/bin/env bash
set -euo pipefail

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required"
  exit 1
fi

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl not found. It will be available after K3s install at /usr/local/bin/kubectl."
fi

echo "[1/3] Install K3s (single-node)"
curl -sfL https://get.k3s.io | sh -

echo "[2/3] Export kubeconfig"
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

echo "[3/3] Verify cluster"
kubectl get nodes -o wide

echo "K3s ready. Next: ./scripts/bootstrap-argocd.sh"
