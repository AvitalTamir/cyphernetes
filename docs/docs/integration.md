---
sidebar_position: 7
---

# Integration

Cyphernetes can be integrated with other tools and programs in various ways. This guide covers different integration methods and provides examples for each approach.

## Go Integration

### Using the Cyphernetes Go Package

Import and use Cyphernetes in your Go programs:

```go
package main

import (
    "context"
    "fmt"
    "github.com/avitaltamir/cyphernetes/pkg/core"
    "github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

func main() {
    // Initialize the Kubernetes provider
    provider, err := apiserver.NewProvider()
    if err != nil {
        panic(err)
    }

    // Create a new Cyphernetes engine
    engine := core.NewEngine(provider)

    // Execute a query
    query := `
        MATCH (p:Pod)
        WHERE p.status.phase != "Running"
        RETURN p.metadata.name, p.status.phase
    `
    
    result, err := engine.Execute(context.Background(), query)
    if err != nil {
        panic(err)
    }

    // Process results
    for _, row := range result.Rows {
        fmt.Printf("Pod: %s, Status: %s\n", row[0], row[1])
    }
}
```

### Custom Provider Implementation

Implement your own provider for custom backends:

```go
package custom

import (
    "context"
    "github.com/avitaltamir/cyphernetes/pkg/provider"
)

type CustomProvider struct {
    // Your custom fields
}

func (p *CustomProvider) GetResources(ctx context.Context, kind string) ([]provider.Resource, error) {
    // Implement resource retrieval
}

func (p *CustomProvider) CreateResource(ctx context.Context, resource provider.Resource) error {
    // Implement resource creation
}

func (p *CustomProvider) UpdateResource(ctx context.Context, resource provider.Resource) error {
    // Implement resource update
}

func (p *CustomProvider) DeleteResource(ctx context.Context, resource provider.Resource) error {
    // Implement resource deletion
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