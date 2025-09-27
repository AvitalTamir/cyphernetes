# Cyphernetes Development Instructions

Cyphernetes is a Cypher-inspired query language for Kubernetes that includes a CLI tool, web interface, and Kubernetes operator. The project is a Go monorepo with multiple components.

**Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.**

## Working Effectively

### Prerequisites and Installation
- Go 1.23+ (check with `go version`)
- Node.js 20+ (check with `node --version`)
- pnpm 9+ (install with `npm install -g pnpm@9`)
- Make (for running make commands)
- Kind (for testing with Kubernetes clusters)

### Bootstrap and Build Process
Run these commands in order to build the complete project:

```bash
# Install pnpm if not available
npm install -g pnpm@9

# Build operator manifests - takes ~50 seconds. NEVER CANCEL. Set timeout to 90+ seconds.
make operator-manifests

# Build web client - takes ~53 seconds. NEVER CANCEL. Set timeout to 90+ seconds.
make web-build

# Build main binary - takes ~71 seconds. NEVER CANCEL. Set timeout to 120+ seconds.
make build

# OR: Build everything at once - takes ~2.5 minutes total. NEVER CANCEL. Set timeout to 180+ seconds.
make
```

### Build Individual Components
- **Main CLI binary**: `make build` (71 seconds)
- **Web client only**: `make web-build` (53 seconds) 
- **Kubectl plugin**: `make build-kubectl-plugin` (2 seconds)
- **Operator manifests**: `make operator-manifests` (50 seconds)

### Testing

#### Working Tests (No Kubernetes Required)
- **Core package tests**: `go test ./pkg/core` (takes <1 second, always works)
- **Web tests**: `make web-test` (takes 4 seconds, always works)

#### Tests Requiring Kubernetes
- **Full test suite**: `make test` (20 seconds, FAILS without Kubernetes cluster)
- **E2E tests**: Tests in `./pkg/core/e2e` require Kubernetes cluster
- **Operator tests**: `make operator-test` (requires setup-envtest and Kubernetes)

**CRITICAL**: Never run `make test` without a Kubernetes cluster. It will fail with "no configuration has been provided" errors.

## Validation and Manual Testing

### Manual Validation Scenarios
After making changes, ALWAYS test these scenarios:

#### Basic CLI Functionality (No Kubernetes)
```bash
# Verify binary builds and basic commands work
./dist/cyphernetes --help
./dist/cyphernetes version
./dist/kubectl-cypher --help
```

#### With Kubernetes Cluster
Set up a test cluster first:
```bash
# Create kind cluster - takes ~31 seconds. NEVER CANCEL.
kind create cluster --name test-cyphernetes

# Verify connectivity
kubectl get nodes

# Test basic query functionality
./dist/cyphernetes query "MATCH (n:Node) RETURN n.metadata.name"

# Test kubectl plugin
./dist/kubectl-cypher "MATCH (n:Node) RETURN n.metadata.name"

# Test web interface (will start on localhost:8080)
./dist/cyphernetes web
# Press Ctrl+C to stop

# Clean up
kind delete cluster --name test-cyphernetes
```

#### Web Interface Validation
The web interface takes ~30 seconds to start (schema resolution) then serves on localhost:8080. You can interact with it through a browser if available, but startup without errors validates the build.

## Project Structure

```
.
├── cmd/                    # CLI applications
│   ├── cyphernetes/       # Main CLI binary source
│   └── kubectl-cypher/    # Kubectl plugin source
├── docs/                  # Docusaurus documentation site
├── operator/              # Kubernetes operator (separate Go module)
├── pkg/
│   ├── core/             # Core Cyphernetes engine and parser
│   └── provider/         # Backend implementations (apiserver)
├── web/                  # React/TypeScript web client (Vite + pnpm)
└── dist/                 # Built binaries (created by make build)
```

## Common Issues and Solutions

### Build Failures
- **"pnpm not found"**: Run `npm install -g pnpm@9`
- **Web build fails**: Ensure Node.js 20+ and pnpm 9+ are installed
- **Go build fails**: Ensure Go 1.23+ is available

### Test Failures
- **"no configuration has been provided"**: Normal when no Kubernetes cluster is available
- **Network failures in tests**: Some tests try to reach `ascii.cyphernet.es` for graph rendering
- **E2E test failures**: Require Kubernetes cluster setup with Kind or similar

### Runtime Issues
- **Web interface fails to start**: Requires valid Kubernetes configuration
- **Query failures**: Ensure kubectl can connect to a cluster
- **"current context does not exist"**: No Kubernetes config available

## CI/CD Integration

The project uses GitHub Actions for CI:
- **PR Checks** (`.github/workflows/pr-checks.yml`): Runs all tests including operator tests
- **Release** (`.github/workflows/release.yml`): Builds multi-platform binaries and Docker images

Before committing changes:
- Run `make` to build everything
- Run `go test ./pkg/core` to verify core functionality
- Run `make web-test` to verify web components
- Test manual scenarios with a Kind cluster if making query engine changes

## Development Tips

### Quick Development Cycle
1. Make code changes
2. Run `make build` (builds web + binary)
3. Test with `./dist/cyphernetes --help`
4. For query changes, test with Kind cluster

### Key Files to Monitor
- **Main CLI**: `cmd/cyphernetes/main.go`, `cmd/cyphernetes/query.go`
- **Core engine**: `pkg/core/` (parser, executor)
- **Web client**: `web/src/` (React components)
- **Build system**: `Makefile`, `web/package.json`

### Performance Notes
- Initial builds download dependencies (~1-2 minutes total)
- Subsequent builds are faster (30-60 seconds)
- Web interface startup includes schema resolution (30 seconds)
- Core tests run in <1 second, web tests in ~4 seconds

**Remember**: Always wait for builds to complete. Build times are normal and expected. Use generous timeouts (90+ seconds for individual components, 180+ seconds for full builds).