# Development Guide

## Prerequisites

- [Podman](https://podman.io/) (or Docker with alias)
- Git
- Make

No Go installation required - everything runs in containers.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/tomblancdev/stromboli-go.git
cd stromboli-go

# Build development container
make build-image

# Run tests
make test

# Run linter
make lint
```

## Available Commands

| Command | Description |
|---------|-------------|
| `make help` | Show all available commands |
| `make build-image` | Build the development container |
| `make shell` | Open interactive shell in container |
| `make build` | Compile the SDK |
| `make test` | Run unit tests |
| `make test-race` | Run tests with race detector |
| `make test-coverage` | Generate coverage report |
| `make test-e2e` | Run E2E tests (requires Stromboli) |
| `make lint` | Run golangci-lint |
| `make fmt` | Format code |
| `make vet` | Run go vet |
| `make generate` | Regenerate code from OpenAPI |
| `make deps` | Download and tidy dependencies |
| `make clean` | Remove build artifacts |

## Code Generation

When the Stromboli API changes:

1. Update `stromboli.yaml`:
   ```yaml
   apiVersion: "0.4.0-alpha"  # New version
   ```

2. Regenerate:
   ```bash
   make generate
   ```

3. Run tests:
   ```bash
   make test
   ```

4. Update wrapper if needed (new endpoints, changed types)

## Testing

### Unit Tests

```bash
make test
```

Tests live in `tests/unit/` and test the wrapper layer with mocked HTTP.

### E2E Tests

```bash
# Start Stromboli locally first
make test-e2e
```

E2E tests live in `tests/e2e/` and require a running Stromboli instance.

### Coverage

```bash
make test-coverage
# Open coverage.html in browser
```

Target: 80%+ coverage on wrapper code.

## Code Style

### Formatting

```bash
make fmt
```

### Linting

```bash
make lint
```

Configuration in `.golangci.yml`. Key linters:
- `errcheck` - Check error handling
- `govet` - Go vet checks
- `staticcheck` - Static analysis
- `revive` - Style checks

### Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Package | lowercase | `stromboli` |
| Exported | PascalCase | `Client`, `RunRequest` |
| Unexported | camelCase | `doRequest` |
| Files | snake_case | `client_test.go` |
| Test functions | `Test` prefix | `TestClientRun` |

## Adding Features

1. **Check if it's an API change** → Update stromboli.yaml + regenerate
2. **Wrapper change** → Edit client.go, add tests
3. **New option** → Add to options.go
4. **New error** → Add to errors.go

## Debugging

### Container Shell

```bash
make shell
# Now inside container:
go test -v ./... -run TestSpecificTest
```

### Verbose Output

```bash
podman run --rm -v $(pwd):/app:Z -w /app stromboli-go-dev \
    go test -v ./...
```
