# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /workspace

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the operator binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o manager cmd/main.go

# Runtime stage — distroless for minimal attack surface
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
