
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o webhook-server ./

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/webhook-server .
EXPOSE 8443
ENTRYPOINT ["/app/webhook-server"]
