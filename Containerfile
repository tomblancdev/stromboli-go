FROM docker.io/golang:1.24-alpine

WORKDIR /app

# Install system dependencies
RUN apk add --no-cache nodejs npm

# Install Go tools
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && \
    go install github.com/go-swagger/go-swagger/cmd/swagger@v0.31.0 && \
    go install github.com/vektra/mockery/v2@latest

# Install prism (OpenAPI mock server)
RUN npm install -g @stoplight/prism-cli

# Copy module files first for caching
COPY go.mod go.sum* ./
RUN go mod download || true

# Copy source
COPY . .

CMD ["go", "test", "./..."]
