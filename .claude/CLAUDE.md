# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Cyphernetes is a Cypher-inspired query language for Kubernetes that transforms complex kubectl commands into intuitive graph-based queries. It's a monorepo containing:
- Go-based query engine and CLI tool
- React web interface for interactive queries  
- Kubernetes operator for dynamic workflows
- kubectl plugin for seamless integration

## Key Commands

### Building and Testing

**Full build (includes operator manifests and web assets):**
```bash
make all
```

**Build specific components:**
```bash
make build                    # Main CLI binary (includes web build)
make build-kubectl-plugin     # kubectl-cypher plugin
make operator-build           # Operator binary
make web-build               # Web UI only
```

**Run tests:**
```bash
make test                    # All Go unit tests
make test-e2e                # End-to-end tests with Ginkgo
make coverage                # Generate and open coverage report
make web-test                # Web UI tests (Vitest)
make operator-test           # Operator unit and e2e tests
```

**Run a single test:**
```bash
# Go tests
go test ./pkg/core -run TestSpecificFunction

# Web tests
cd web && pnpm test -- testname

# Operator tests
cd operator && go test ./internal/controller -run TestSpecificController
```

**Development server:**
```bash
# Web UI development with hot reload
cd web && pnpm dev

# Run the built web UI
make web-run
```

**Clean build artifacts:**
```bash
make clean
```

## Architecture

### Core Components

**`/cmd/cyphernetes`** - Main CLI application
- `main.go` - Entry point, calls Execute()
- `root.go` - Cobra CLI setup with subcommands (shell, query, web, operator, macro, graph, api)
- `shell.go` - Interactive shell implementation
- `web/` - Embedded web UI assets (built from /web)

**`/pkg/core`** - Query language implementation
- `parser/` - Cypher-like syntax parser
- `engine.go` - Query execution engine
- `relationships.go` - Kubernetes resource relationship mappings
- `e2e/` - End-to-end test suite using Ginkgo

**`/pkg/provider`** - Backend abstraction
- `apiserver/` - Kubernetes API server client implementation

**`/web`** - React-based web interface
- Built with Vite, TypeScript, and React
- Uses Vitest for testing with React Testing Library
- Graph visualization with react-force-graph-2d

**`/operator`** - Kubernetes operator
- CRDs for DynamicOperator resources
- Controllers for executing Cyphernetes queries as workflows
- Helm chart for deployment

### Key Design Patterns

1. **Relationship Mapping**: The arrow syntax (`->`) leverages predefined relationships between Kubernetes resources (e.g., Deployment -> Pod, Service -> Endpoint)

2. **Multi-cluster Support**: Queries can target multiple Kubernetes contexts simultaneously

3. **Macro System**: Reusable query templates stored in `~/.cyphernetes/macros/`

4. **Provider Interface**: Abstraction layer allows for different backends beyond Kubernetes API server

## Development Workflow

1. **Check formatting and linting** (operator only has golangci-lint configured):
   ```bash
   cd operator && make lint
   ```

2. **Before committing**: Ensure all tests pass
   ```bash
   make test
   make web-test
   make operator-test
   ```

3. **CI/CD**: GitHub Actions runs all tests on PRs, including:
   - Unit tests for all components
   - E2E tests with Kind cluster
   - Operator tests with envtest

4. **Version injection**: Build commands inject version via ldflags
   ```bash
   make build VERSION=v1.2.3
   ```

## Important Notes

- Web assets must be built before the main binary (`make web-build` is called by `make build`)
- The operator requires CRD generation before building (`make operator-manifests`)
- E2E tests require a running Kubernetes cluster (Kind is used in CI)
- User configuration and history stored in `~/.cyphernetes/`
- The project uses Go 1.23.0 with toolchain 1.23.2
- Web development requires Node.js and pnpm 9+

## Claude Memory System

When working on large projects, use the `.claude/` directory for persistent memory:

### Memory Files

**`.claude/PROJECT_MEMORY.md`** - High-level project tracking
- Current project status and phase
- Key decisions made
- Progress tracking and milestones
- Open questions needing clarification
- Technical notes and discoveries
- Dependencies and blockers

**`.claude/DESIGN_NOTES.md`** - Detailed design documentation
- Architecture designs and diagrams
- API specifications
- Data models and structures
- Integration points with existing code
- Performance and security considerations

**`.claude/IMPLEMENTATION_LOG.md`** - Implementation diary
- Session-by-session progress log
- Code changes made
- Tests written
- Issues encountered and solutions
- Next steps for each session

### Usage Guidelines

1. **Start of session**: Read all memory files to understand context
2. **During work**: Update files as you make progress or decisions
3. **End of session**: Summarize progress in IMPLEMENTATION_LOG.md
4. **Design changes**: Document in DESIGN_NOTES.md before implementing
5. **Questions/blockers**: Track in PROJECT_MEMORY.md for follow-up