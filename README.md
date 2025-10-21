# Mutating Webhook Example

This project implements a simple Kubernetes mutating admission webhook in Go 1.24. It adds the label `hello: world` to all pods if the label does not already exist.

## Files
- `main.go`: Webhook server implementation
- `deploy.yaml`: Deployment and Service manifest
- `webhook.yaml`: MutatingWebhookConfiguration manifest
- `generate-certs.sh`: Script to generate self-signed TLS certificates

## Quick Start

1. Generate TLS certificates:
   ```sh
   ./generate-certs.sh
   ```
2. Build and containerize the webhook server, push to your registry, and update the image in `deploy.yaml`.
3. Apply the manifests:
   ```sh
   kubectl apply -f deploy.yaml
   kubectl apply -f webhook.yaml
   ```
4. Make sure to replace `<CA_BUNDLE>` in `webhook.yaml` with the base64-encoded contents of `tls.crt`.

## Requirements
- Go 1.24
- Kubernetes cluster

---

This is a minimal example for educational/demo purposes. For production, add authentication, logging, and error handling improvements.
