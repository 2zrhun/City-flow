#!/usr/bin/env bash
set -euo pipefail

echo "[1/5] Create argocd namespace"
kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -

echo "[2/5] Install Argo CD"
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

echo "[3/5] Wait for Argo CD server"
kubectl -n argocd rollout status deploy/argocd-server --timeout=300s

echo "[4/5] Apply CityFlow project and application"
kubectl apply -f argocd/project-cityflow.yaml
kubectl apply -f argocd/application-cityflow.yaml

echo "[5/5] Done"
echo "Argo CD UI:"
echo "  kubectl -n argocd port-forward svc/argocd-server 8081:443"
echo "Initial admin password:"
echo "  kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d; echo"
