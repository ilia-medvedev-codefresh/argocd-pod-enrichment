
# Check if the k3d cluster 'webhook' exists, create if not
if ! k3d cluster list webhook | grep -q '^webhook\b'; then
	k3d cluster create webhook --registry-use k3d-registry.localhost:5050
fi

k3d kubeconfig show webhook > $(dirname "$0")/../../kubeconfig.yaml
export KUBECONFIG=$(dirname "$0")/../../kubeconfig.yaml
helm --repo https://charts.crossplane.io/stable upgrade --install crossplane crossplane --namespace crossplane-system --create-namespace --wait
kubectl apply -f $(dirname "$0")/../../manifests/webhook
kubectl apply -f $(dirname "$0")/../../manifests/controller
helm -n argocd upgrade --install argocd oci://ghcr.io/argoproj/argo-helm/argo-cd --values $(dirname "$0")/argocd-values.yaml --version 9.0.3 --create-namespace --wait
kubectl -n argocd apply -f $(dirname "$0")/guestbook.yaml
kubectl -n argocd-apps apply -f $(dirname "$0")/guestbook-2.yaml
