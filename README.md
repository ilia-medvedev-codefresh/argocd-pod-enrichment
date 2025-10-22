# ArgoCD Pod Enrichment Webhook

This project implements a Kubernetes mutating admission webhook in Go. The webhook command propagates ArgoCD ownership information (application name, namespace, and installation ID) from any top-level resource down to a pod by adding the relevant labels to the pod.

## Features
- Extracts ArgoCD tracking information from the topmost owner of a pod (e.g., Deployment, StatefulSet, etc.).
- Adds or updates the following labels on the pod:
  - Application name
  - Application namespace
  - Installation ID

## Usage

### Mutating webhook

```sh
argocd-pod-enrichment webhook --tls-cert <cert> --tls-key <key> --port <port>
```

- `--tls-cert`: Path to the TLS certificate file (default: `/certs/tls.crt`)
- `--tls-key`: Path to the TLS private key file (default: `/certs/tls.key`)
- `--port`: Port to listen on for HTTPS traffic (default: `8443`)

### Example Deployment

1. Build and containerize the webhook server, push to your registry, and update the image in your deployment manifest.
2. Apply the manifests for the webhook deployment and `MutatingWebhookConfiguration`.
3. Ensure the webhook has access to the Kubernetes API and the necessary RBAC permissions.

## Requirements
- Go 1.24
- Kubernetes cluster

---

This webhook is designed to work with ArgoCD-managed resources and will automatically propagate ArgoCD application ownership labels to pods, making it easier to track and manage workloads in your cluster.
