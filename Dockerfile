# Build stage
FROM golang:1.21-alpine AS builder

# Install git for fetching dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gcp_footprint .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 -S appuser && \
    adduser -u 1000 -S appuser -G appuser

# Set working directory
WORKDIR /output

# Copy binary from builder
COPY --from=builder /app/gcp_footprint /usr/local/bin/gcp_footprint

# Change ownership of output directory
RUN chown -R appuser:appuser /output

# Switch to non-root user
USER appuser

# Set entry point
ENTRYPOINT ["gcp_footprint"]
