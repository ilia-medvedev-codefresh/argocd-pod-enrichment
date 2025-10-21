#!/bin/bash
set -e
cd "$(dirname "$0")"

CERTS_DIR="../certs"
MANIFESTS_DIR="../manifests"
WEBHOOK_YAML="$MANIFESTS_DIR/webhook.yaml"
SECRET_YAML="$MANIFESTS_DIR/secret.yaml"
CONF_FILE="webhook-csr.conf"

mkdir -p "$CERTS_DIR"

# 1. Generate CA key and certificate
openssl genpkey -algorithm RSA -out "$CERTS_DIR/ca-key.pem"
openssl req -x509 -new -nodes -key "$CERTS_DIR/ca-key.pem" -days 365 -out "$CERTS_DIR/ca-cert.pem" -subj "/CN=Webhook CA"

# 2. Generate server key and CSR with SANs
openssl genpkey -algorithm RSA -out "$CERTS_DIR/server-key.pem"
openssl req -new -key "$CERTS_DIR/server-key.pem" -out "$CERTS_DIR/server.csr" -config "$CONF_FILE" -subj "/CN=mutating-webhook-service.default.svc"

# 3. Sign server certificate with CA
openssl x509 -req -in "$CERTS_DIR/server.csr" -CA "$CERTS_DIR/ca-cert.pem" -CAkey "$CERTS_DIR/ca-key.pem" -CAcreateserial -out "$CERTS_DIR/server-cert.pem" -days 365 -extensions v3_req -extfile "$CONF_FILE"

# 4. Create Kubernetes TLS secret manifest
echo "Creating Kubernetes TLS secret manifest..."
cat > "$SECRET_YAML" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: webhook-server-tls
  namespace: default
type: kubernetes.io/tls
data:
  tls.crt: $(base64 -i "$CERTS_DIR/server-cert.pem")
  tls.key: $(base64 -i "$CERTS_DIR/server-key.pem")
EOF

# 5. Base64 encode the CA cert for webhook.yaml
CA_BUNDLE=$(base64 -i "$CERTS_DIR/ca-cert.pem")
export CA_BUNDLE
if command -v yq >/dev/null 2>&1; then
  yq e '.webhooks[0].clientConfig.caBundle = env(CA_BUNDLE)' -i "$WEBHOOK_YAML"
else
  echo "yq not found. Please install yq for robust YAML editing. Falling back to sed."
  sed -i.bak "s|caBundle:.*|caBundle: $CA_BUNDLE|" "$WEBHOOK_YAML"
fi

echo "Certificates and manifests generated."
