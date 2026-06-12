# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o gatherhub ./cmd/server/main.go

# ---

# Runtime stage
FROM alpine:3.20

WORKDIR /app

# Install runtime deps
RUN apk --no-cache add ca-certificates tzdata

# Set timezone
ENV TZ=Asia/Jakarta

# Copy binary from builder
COPY --from=builder /build/gatherhub .

# Create uploads directory
RUN mkdir -p /app/uploads

# Non-root user
RUN addgroup -S gatherhub && adduser -S gatherhub -G gatherhub
RUN chown -R gatherhub:gatherhub /app
USER gatherhub

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:3000/health || exit 1

CMD ["./gatherhub"]
