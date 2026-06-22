# Multi-stage build for the relay server
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o nod-relay ./cmd/nod-relay

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/nod-relay /usr/local/bin/nod-relay
RUN mkdir -p /data
VOLUME /data
EXPOSE 55558
CMD ["nod-relay", "--port", "55558"]
