# Multi-stage build for the relay server
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o termtalk-relay ./cmd/termtalk-relay

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/termtalk-relay /usr/local/bin/termtalk-relay
EXPOSE 55558
CMD ["termtalk-relay", "--port", "55558"]
