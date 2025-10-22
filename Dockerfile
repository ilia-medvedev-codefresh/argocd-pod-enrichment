
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o argocd-pod-enrichment ./

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/argocd-pod-enrichment .
ENTRYPOINT ["/app/argocd-pod-enrichment"]
