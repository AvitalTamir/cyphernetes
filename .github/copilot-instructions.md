# Cyphernetes - GitHub Copilot Instructions

## Project Overview

Cyphernetes is a Cypher-inspired query language for Kubernetes that allows users to express Kubernetes graph operations using ASCII-art, SQL-like syntax, and JSONPath. It works out-of-the-box with CRDs, supports multi-cluster queries, and provides multiple interfaces including a web client, interactive shell, CLI, kubectl plugin, and Kubernetes operator.

## Repository Structure

This is a monorepo with multiple components:

```
.
├── cmd/cyphernetes/           # CLI binary and main entry point
├── cmd/kubectl-cypher/        # kubectl plugin implementation
├── pkg/
│   ├── core/                  # Core Cyphernetes package (parser and engine)
│   └── provider/              # Interface for backend implementations
│       └── apiserver/         # Kubernetes API server client
├── web/                       # React/TypeScript web client (Vite + pnpm)
├── operator/                  # Kubernetes operator (separate Go module)
└── docs/                      # Documentation website
```

## Technology Stack

### Backend
- **Language**: Go 1.23+
- **Frameworks**: 
  - Cobra (CLI)
  - Gin (web server)
  - controller-runtime (Kubernetes operator)
- **Testing**: Ginkgo/Gomega for BDD-style tests
- **Package Manager**: Go modules

### Frontend
- **Language**: TypeScript
- **Framework**: React 18
- **Build Tool**: Vite
- **Package Manager**: pnpm 9+
- **Testing**: Vitest

### Operator
- **Framework**: Kubebuilder/controller-runtime
- **Linting**: golangci-lint with custom configuration

## Development Commands

### Build Commands
```bash
make                    # Build all components (operator manifests, web, binary, tests)
make build             # Build main binary (includes web build)
make build-kubectl-plugin    # Build kubectl plugin
make web-build         # Build web client only
make operator-manifests      # Generate operator manifests
```

### Test Commands
```bash
make test              # Run Go tests
make web-test          # Run web client tests
make operator-test     # Run operator tests
make test-e2e          # Run e2e tests (requires Kind cluster)
make coverage          # Generate coverage report
```

### Development Workflow
```bash
make clean             # Clean build artifacts
make web-run           # Run web development server
cyphernetes shell      # Launch interactive shell (after building)
cyphernetes web        # Launch web UI (after building)
```

## Coding Conventions

### Go Code
- Follow standard Go formatting (gofmt, goimports)
- Use golangci-lint for linting (see operator/.golangci.yml)
- Write tests using Ginkgo/Gomega for BDD-style tests
- Standard Go tests are also acceptable
- Keep functions focused and modular
- Error handling: return errors, don't panic
- Use meaningful variable names (avoid single letters except in short scopes)

### TypeScript/React Code
- Use TypeScript for all new code
- Follow React functional components with hooks
- Use Vitest for testing with @testing-library/react
- Keep components small and focused
- Use proper TypeScript types (avoid 'any')

### Testing
- E2E tests require a Kubernetes cluster (Kind is used in CI)
- Unit tests should not require cluster access
- Test files use `_test.go` suffix
- Web tests use `.test.tsx` or `.test.ts` suffix

## Key Features to Understand

1. **Query Language**: Cypher-inspired syntax for Kubernetes operations
2. **Relationships**: Understanding connections between K8s resources (using `->` arrows)
3. **Multi-context**: Support for multiple Kubernetes clusters
4. **CRD Support**: Works automatically with Custom Resource Definitions
5. **Graph Operations**: MATCH, CREATE, SET, DELETE operations on K8s resources

## Common Patterns

### Parser and Lexer
- Core query parsing is in `pkg/core/parser.go` and `pkg/core/lexer.go`
- Abstract Syntax Tree (AST) is defined in `pkg/core/types.go`

### Query Execution
- `QueryExecutor` in `pkg/core/engine.go` is the main execution engine
- Provider interface abstracts K8s API interactions
- Results are cached for performance

### Web API
- Gin router in `cmd/cyphernetes/` handles web requests
- Web client communicates via REST API
- Static assets are embedded in the binary

## Important Notes

### Building
- Web assets must be built before the main binary (handled by Makefile)
- Operator manifests must be generated before tests
- pnpm is required for web builds (not npm)

### Testing Limitations
- E2E tests fail without a Kubernetes cluster
- Some tests require KUBEBUILDER_ASSETS environment variable
- Network-dependent tests (ASCII art conversion) may fail in restricted environments

### Dependencies
- Go dependencies are managed per module (root and operator/)
- Web dependencies use pnpm with lockfile
- Always run `pnpm install` in web/ before building

## CI/CD
- PR checks run on GitHub Actions
- Tests run against Kind cluster
- Requires Go, Node.js, pnpm, and Kind setup
- See `.github/workflows/pr-checks.yml` for full pipeline

## Contributing Guidelines

1. Make minimal, focused changes
2. Run tests before submitting PRs
3. Follow existing code style and conventions
4. Update documentation if changing user-facing features
5. Web changes require running `make web-build` before committing
6. Operator changes may require regenerating manifests with `make operator-manifests`

## Architecture Insights

### Query Flow
1. User input → Lexer → Tokens
2. Tokens → Parser → AST
3. AST → QueryExecutor → K8s API calls
4. Results → Cache → Formatted output

### Relationship Types
- Defined in `pkg/core/relationship_types.go`
- Automatically inferred from K8s resource schemas
- Examples: Pod → Service, Deployment → ReplicaSet → Pod

### Multi-cluster Support
- Each context gets its own QueryExecutor
- Context switching is transparent to users
- Configured via kubeconfig

## Performance Considerations

- Results are aggressively cached
- Concurrent API requests use semaphores for rate limiting
- Graph operations are optimized for common patterns
- JSONPath evaluation is cached

## Security Notes

- Always respect kubeconfig permissions
- No credentials are stored by Cyphernetes
- Operator requires RBAC configuration
- Web server binds to localhost by default
