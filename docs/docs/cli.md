---
sidebar_position: 4
---

# CLI Guide

The Cyphernetes CLI provides multiple ways to interact with your Kubernetes clusters using the Cypher-inspired query language.

## Command Overview

Cyphernetes provides several main commands:

```bash
cyphernetes [command] [flags]
```

Available commands:
- `web` - Start the web interface
- `shell` - Start an interactive shell
- `query` - Execute a single query
- `version` - Show version information

## Web Interface

The web interface provides a graphical environment for writing and executing queries:

```bash
cyphernetes web [flags]
```

Options:
- `--port` - Port to listen on (default: 8080)

After starting the web interface, visit `http://localhost:8080` in your browser.

## Interactive Shell

The interactive shell provides a REPL (Read-Eval-Print Loop) environment for executing queries:

```bash
cyphernetes shell [flags]
```

Features:
- Command history (use arrow keys to navigate)
- Tab completion for Cypher keywords
- Multi-line query support
- Query result formatting

Example session:
```bash
$ cyphernetes shell
cyphernetes> MATCH (p:Pod) RETURN p.metadata.name;
NAME
nginx-deployment-6b474476c4-2p8l7
nginx-deployment-6b474476c4-9x8k2
...

cyphernetes> MATCH (d:Deployment)
cyphernetes> WHERE d.metadata.name = "nginx"
cyphernetes> RETURN d;
...
```

## Single Query Execution

Execute a single query directly from the command line:

```bash
cyphernetes query "MATCH (p:Pod) RETURN p.metadata.name"

# Delete failed pods
cyphernetes query "MATCH (p:Pod) WHERE p.status.phase = 'Failed' DELETE p"
```

Options:
- `--format` - Output format (json, yaml, table)
- `--namespace, -n` - Kubernetes namespace

## Output Formatting

Control the output format of your queries:

```bash
# Output as JSON
cyphernetes query "MATCH (p:Pod) RETURN p"

# Output as YAML
cyphernetes query --format yaml "MATCH (p:Pod) RETURN p"
```

## Shell Scripting

Cyphernetes can be used effectively in shell scripts:

```bash
#!/bin/bash

# Get all non-running pods
FAILED_PODS=$(cyphernetes query \
  "MATCH (p:Pod) WHERE p.status.phase != 'Running' RETURN p.metadata.name")

# Process the results
echo $FAILED_PODS | jq -r '.[]' | while read pod; do
  echo "Found non-running pod: $pod"
done
```

## Environment Variables

Cyphernetes respects the following environment variables:

- `KUBECONFIG` - Path to kubeconfig file

## Best Practices

1. **Use the Web Interface** for exploring and developing queries
2. **Use the Shell** for interactive debugging and quick queries
3. **Use Query Command** for automation and scripting
4. **Set Default Context** when working with multiple clusters
5. **Use Output Formatting** appropriate for your use case 