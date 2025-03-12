---
sidebar_position: 7
---

# Integration

Cyphernetes can be integrated with other tools and programs in various ways. This guide covers different integration methods and provides examples for each approach.

## Go Integration

Cyphernetes is made up of two main packages:

1. The `pkg/core` package, which contains the Cyphernetes parser and engine.
2. The `pkg/provider` package, which contains the Cyphernetes provider interface and a default implementation for an api-server client.

### Using the Cyphernetes Go Package

Import and use Cyphernetes in your Go programs:

```go
package main

import (
    "fmt"
    "log"
    "github.com/avitaltamir/cyphernetes/pkg/core"
    "github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

func main() {
    // Initialize the Kubernetes provider
    provider := apiserver.NewAPIServerProvider()
    
    // Create a new Cyphernetes query executor
    executor := core.NewQueryExecutor(provider)
    
    // Execute a query
    query := "MATCH (p:Pod) WHERE p.status.phase != 'Running' RETURN p.metadata.name"
    
    result, err := executor.Parse(query)
    if err != nil {
        log.Fatalf("Error parsing query: %v", err)
    }
    
    // Process results
    fmt.Printf("Result: %+v\n", result)
}
```

### Custom Provider Implementation

Implement your own provider for custom backends by implementing the Provider interface defined in `pkg/provider/interface.go`:

```go
type Provider interface {
    // Resource Operations
    GetK8sResources(kind, fieldSelector, labelSelector, namespace string) (interface{}, error)
    DeleteK8sResources(kind, name, namespace string) error
    CreateK8sResource(kind, name, namespace string, body interface{}) error
    PatchK8sResource(kind, name, namespace string, body interface{}) error

    // Schema Operations
    FindGVR(kind string) (schema.GroupVersionResource, error)
    GetOpenAPIResourceSpecs() (map[string][]string, error)
    CreateProviderForContext(context string) (Provider, error)
}
```

The 4 CRUD operations are straightforward. They all take a kind and namespace, "Get" operations take a fieldSelector and labelSelector, while "Create" and "Patch" operations take a body (JSON for "Create", and a JSON patch for "Patch").

The schema operation functions are as follows:

- `FindGVR` is used to find the GVR for a given kind. This is used by the Cyphernetes parser to find the correct API endpoint to query.
- `GetOpenAPIResourceSpecs` is used to get a flat list of JSONPaths for a given kind. This is used by the Cyphernetes parser to understand the API schema.
- `CreateProviderForContext` is used to create a new provider for a given context. This is used by the Cyphernetes engine when running multi-context queries only.

You can implement your own provider from scratch, as long as it:
- implements the `Provider` interface
- `FindGVR` correctly resolves strings to `schema.GroupVersionResource` objects
- `GetOpenAPIResourceSpecs` provides a list of JSONPaths for each kind
- You may choose to make this a "read-only" provider by having CUD operations return an error or warning
- You may choose to support multiple Kubernetes contexts, or leave out this functionality and return an error from `CreateProviderForContext`

### Kubernetes Client Integration

The `pkg/provider/apiserver` package is a wrapper around the Kubernetes client-go library and may be used as a base implementation for your own provider. If your program already uses client-go, you can re-use a lot of the code in the `pkg/provider/apiserver` package:

- You can pass a clientSet to the `NewAPIServerProvider` function
- Or you can initialize the provider with no options and a new clientSet will be created from the available configuration

Example of JSONPaths returned by `GetOpenAPIResourceSpecs()`:

```
{
    ...
    "pods": [ // Plural name of the kind
        ...
        "$.metadata.name",
        "$.spec.containers[*].name",
        "$.spec.containers[*].resources.requests.cpu",
        "$.spec.containers[*].resources.requests.memory",
        "$.spec.containers[*].resources.limits.cpu",
        "$.spec.containers[*].resources.limits.memory"
        ...
    ],
    ...
}
```

## CI/CD Integration

### GitHub Actions

Use Cyphernetes in your GitHub Actions workflows:

```yaml
name: Kubernetes Cleanup
on:
  schedule:
    - cron: '0 0 * * *'  # Run daily at midnight

jobs:
  cleanup:
    runs-on: ubuntu-latest
    steps:
      - name: Install Cyphernetes
        run: |
          curl -LO https://github.com/avitaltamir/cyphernetes/releases/latest/download/cyphernetes-linux-amd64
          chmod +x cyphernetes-linux-amd64
          sudo mv cyphernetes-linux-amd64 /usr/local/bin/cyphernetes

      - name: Configure Kubernetes
        uses: azure/k8s-set-context@v1
        with:
          kubeconfig: ${{ secrets.KUBECONFIG }}

      - name: Clean up failed pods
        run: |
          cyphernetes query '
            MATCH (p:Pod)
            WHERE p.status.phase = "Failed"
              AND p.status.startTime < datetime() - duration("P7D")
            DELETE p;
          '
```

### GitLab CI

Integrate with GitLab CI pipelines:

```yaml
cleanup_job:
  image: alpine
  script:
    - apk add --no-cache curl
    - curl -LO https://github.com/avitaltamir/cyphernetes/releases/latest/download/cyphernetes-linux-amd64
    - chmod +x cyphernetes-linux-amd64
    - mv cyphernetes-linux-amd64 /usr/local/bin/cyphernetes
    - echo "$KUBECONFIG" > kubeconfig.yaml
    - export KUBECONFIG=kubeconfig.yaml
    - |
      cyphernetes query '
        MATCH (p:Pod)
        WHERE p.status.phase = "Failed"
        DELETE p;
      '
  only:
    - schedules
```

## Monitoring Integration

### Prometheus Integration

Export Cyphernetes metrics to Prometheus:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: cyphernetes-operator
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: cyphernetes-operator
  endpoints:
    - port: metrics
```

### Grafana Dashboards

Example Grafana dashboard configuration:

```json
{
  "panels": [
    {
      "title": "Query Execution Time",
      "targets": [
        {
          "expr": "rate(cyphernetes_operator_query_duration_seconds_sum[5m])",
          "legendFormat": "{{query}}"
        }
      ]
    },
    {
      "title": "Error Rate",
      "targets": [
        {
          "expr": "rate(cyphernetes_operator_errors_total[5m])",
          "legendFormat": "{{type}}"
        }
      ]
    }
  ]
}
```

## Shell Integration

### Shell Scripts

Create shell functions for common operations:

```bash
#!/bin/bash

# Function to clean up resources
cleanup_resources() {
    local namespace=$1
    local age=$2
    
    cyphernetes query "
        MATCH (p:Pod)
        WHERE p.metadata.namespace = '$namespace'
          AND p.status.startTime < datetime() - duration('$age')
          AND p.status.phase in ['Failed', 'Succeeded']
        DELETE p;
    "
}

# Function to check deployment status
check_deployments() {
    local namespace=$1
    
    cyphernetes query "
        MATCH (d:Deployment)
        WHERE d.metadata.namespace = '$namespace'
          AND d.status.availableReplicas != d.spec.replicas
        RETURN d.metadata.name,
               d.spec.replicas as desired,
               d.status.availableReplicas as actual;
    "
}
```

### Aliases

Add useful aliases to your shell configuration:

```bash
# ~/.bashrc or ~/.zshrc

# Quick pod listing
alias kpods="cyphernetes query 'MATCH (p:Pod) RETURN p.metadata.name, p.status.phase'"

# Find failed resources
alias kfailed="cyphernetes query 'MATCH (p:Pod) WHERE p.status.phase = \"Failed\" RETURN p'"

# Check deployment status
alias kstatus="cyphernetes query 'MATCH (d:Deployment) RETURN d.metadata.name, d.status.availableReplicas, d.spec.replicas'"
```

## API Integration

### REST API

Use the Cyphernetes HTTP API:

```python
import requests
import json

def execute_query(query):
    response = requests.post(
        'http://localhost:8080/api/v1/query',
        json={'query': query}
    )
    return response.json()

# Example usage
result = execute_query("""
    MATCH (p:Pod)
    WHERE p.status.phase != "Running"
    RETURN p.metadata.name, p.status.phase
""")

for row in result['data']:
    print(f"Pod: {row[0]}, Status: {row[1]}")
```

### WebSocket API

Connect to the real-time WebSocket API:

```javascript
const WebSocket = require('ws');

const ws = new WebSocket('ws://localhost:8080/api/v1/watch');

ws.on('open', () => {
    // Subscribe to pod events
    ws.send(JSON.stringify({
        query: `
            MATCH (p:Pod)
            WHERE p.status.phase != "Running"
            RETURN p
        `
    }));
});

ws.on('message', (data) => {
    const event = JSON.parse(data);
    console.log('Resource changed:', event);
});
```

## Best Practices

1. **Error Handling**: Always handle errors and implement retries for network operations
2. **Resource Management**: Close connections and clean up resources properly
3. **Security**: Use secure connections and proper authentication
4. **Monitoring**: Implement monitoring for your integrations
5. **Testing**: Test integrations thoroughly in a non-production environment 