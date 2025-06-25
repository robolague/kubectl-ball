# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o kubectl-ball .

# Runtime stage
FROM alpine:latest

# Install kubectl and fzf
RUN apk add --no-cache \
    curl \
    bash \
    && curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    && chmod +x kubectl \
    && mv kubectl /usr/local/bin/ \
    && curl -L https://github.com/junegunn/fzf/releases/download/0.42.0/fzf-0.42.0-linux_amd64.tar.gz | tar xz \
    && mv fzf /usr/local/bin/ \
    && rm -rf /var/cache/apk/*

# Copy the binary from builder stage
COPY --from=builder /app/kubectl-ball /usr/local/bin/

# Create directory for config
RUN mkdir -p /root/.kubectl-ball

# Set working directory
WORKDIR /workspace

# Default command
ENTRYPOINT ["kubectl-ball"] 