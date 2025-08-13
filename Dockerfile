FROM --platform=$BUILDPLATFORM golang:1.24.5-alpine3.22 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum /build/

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -ldflags="-X main.version=${VERSION}" -o pgdumps3 /build/cmd/pgdumps3

# Final stage
FROM alpine:3.22

# Install dependencies
RUN apk --no-cache add ca-certificates \
    postgresql15-client \
    postgresql16-client \
    postgresql17-client \
    tzdata


RUN addgroup -g 1000 appuser && \
    adduser -D -H -u 1000 -G appuser appuser && \
    mkdir -p /app && \
    chown -R appuser:appuser /app

COPY --from=builder --chown=appuser:appuser /build/pgdumps3 /usr/local/bin/
RUN chmod +x /usr/local/bin/pgdumps3
# Switch to non-root user
USER appuser

# Set working directory
WORKDIR /app

# Run the application
ENTRYPOINT ["pgdumps3"]
