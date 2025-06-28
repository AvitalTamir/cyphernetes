# kubectl-cypher

A lean kubectl plugin for executing Cyphernetes queries against Kubernetes resources.

## Overview

`kubectl-cypher` is a kubectl plugin that provides the query functionality of Cyphernetes in a lightweight, standalone binary. Unlike the main `cyphernetes` binary which includes shell, operator, and web interfaces, `kubectl-cypher` focuses solely on query execution.

## Installation

### Method 1: Via Krew (Recommended)

```bash
kubectl krew install cypher
```

### Method 2: Build from Source

```bash
# Clone the repository
git clone https://github.com/avitaltamir/cyphernetes.git
cd cyphernetes

# Build the kubectl plugin (defaults to version "dev")
make build-kubectl-plugin

# Or build with a specific version
VERSION=1.0.0 make build-kubectl-plugin

# Copy to your PATH (choose a directory in your PATH)
cp dist/kubectl-cypher /usr/local/bin/kubectl-cypher
```

### Method 3: Download Pre-built Binary

Download the appropriate tar.gz package for your platform from the [releases page](https://github.com/avitaltamir/cyphernetes/releases):

```bash
# macOS Intel
curl -L -o kubectl-cypher.tar.gz https://github.com/avitaltamir/cyphernetes/releases/latest/download/kubectl-cypher-darwin-amd64.tar.gz

# macOS Apple Silicon
curl -L -o kubectl-cypher.tar.gz https://github.com/avitaltamir/cyphernetes/releases/latest/download/kubectl-cypher-darwin-arm64.tar.gz

# Linux Intel/AMD64
curl -L -o kubectl-cypher.tar.gz https://github.com/avitaltamir/cyphernetes/releases/latest/download/kubectl-cypher-linux-amd64.tar.gz

# Linux ARM64
curl -L -o kubectl-cypher.tar.gz https://github.com/avitaltamir/cyphernetes/releases/latest/download/kubectl-cypher-linux-arm64.tar.gz

# Windows Intel/AMD64
curl -L -o kubectl-cypher.tar.gz https://github.com/avitaltamir/cyphernetes/releases/latest/download/kubectl-cypher-windows-amd64.tar.gz

# Windows ARM64
curl -L -o kubectl-cypher.tar.gz https://github.com/avitaltamir/cyphernetes/releases/latest/download/kubectl-cypher-windows-arm64.tar.gz

# Extract the package
tar -xzf kubectl-cypher.tar.gz

# Make it executable (Unix-like systems)
chmod +x kubectl-cypher  # or kubectl-cypher.exe on Windows

# Move to your PATH
mv kubectl-cypher /usr/local/bin/kubectl-cypher  # Unix-like systems
# or move kubectl-cypher.exe to a directory in your PATH on Windows

# Clean up
rm kubectl-cypher.tar.gz LICENSE  # Remove the downloaded files
```

### Verify Installation

```bash
kubectl cypher --version
```

## Usage

Once installed, you can use it as a kubectl plugin:

```bash
# Basic pod query
kubectl cypher "MATCH (p:Pod) RETURN p"

# Query with namespace
kubectl cypher -n kube-system "MATCH (p:Pod) RETURN p.metadata.name"

# Query all namespaces
kubectl cypher -A "MATCH (p:Pod) WHERE p.metadata.name CONTAINS 'nginx' RETURN p"

# Output in YAML format
kubectl cypher --format yaml "MATCH (s:Service) RETURN s"

# Dry-run mode
kubectl cypher --dry-run "MATCH (p:Pod) SET p.metadata.labels.new='value' RETURN p"
```

## Available Flags

- `-n, --namespace string`: The namespace to query against (default "default")
- `-A, --all-namespaces`: Query all namespaces
- `--format string`: Output format (json or yaml) (default "json")
- `--dry-run`: Enable dry-run mode for all operations
- `--no-color`: Disable colored output
- `-r, --raw-output`: Disable JSON output formatting
- `-v, --version`: Show version and exit
- `-h, --help`: Show help

## Examples

### List all pods in the current namespace
```bash
kubectl cypher "MATCH (p:Pod) RETURN p.metadata.name, p.status.phase"
```

### Find pods connected to services
```bash
kubectl cypher "MATCH (p:Pod)->(s:Service) RETURN p.metadata.name, s.metadata.name"
```

### Query specific resource types
```bash
kubectl cypher "MATCH (d:Deployment) WHERE d.spec.replicas > 1 RETURN d.metadata.name, d.spec.replicas"
```

### Update resources (dry-run)
```bash
kubectl cypher --dry-run "MATCH (p:Pod) WHERE p.metadata.name = 'my-pod' SET p.metadata.labels.environment = 'production'"
```

## Size Comparison

The `kubectl-cypher` binary is significantly smaller than the full `cyphernetes` binary:

- `kubectl-cypher`: ~64MB (query functionality only)
- `cyphernetes`: ~72MB (includes shell, operator, web interface)

## Documentation

For full Cyphernetes documentation including query syntax and examples, visit:
https://cyphernetes.io

## License

Apache 2.0 - See [LICENSE](../../LICENSE) for details. 