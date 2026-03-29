# Build stage
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build \
    -ldflags="-s -w" \
    -o b2500-meter-go cmd/b2500-meter/main.go

# Final stage
FROM alpine:3.21

# Create a non-root user (available for manual usage via --user 1000)
RUN adduser -D -u 1000 appuser

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/b2500-meter-go .

# Run as root by default to allow binding to privileged ports (like 1010)
# on systems where this is restricted.
EXPOSE 1010/udp 2220/udp

# Default config path
ENTRYPOINT ["./b2500-meter-go"]
CMD ["--config", "config.yaml"]
