---
sidebar_position: 5
---

# Operator

The Cyphernetes Operator extends Kubernetes with dynamic, graph-based automation capabilities. It allows you to define custom resources that use Cyphernetes queries to monitor and manage your cluster.

## Installation

You can install the Cyphernetes Operator using kubectl:

```bash
# Install CRDs and operator
kubectl apply -f https://raw.githubusercontent.com/avitaltamir/cyphernetes/main/dist/install.yaml
```

Or using Helm:

```bash
helm repo add cyphernetes https://avitaltamir.github.io/cyphernetes/charts
helm repo update
helm install cyphernetes-operator cyphernetes/operator
```

## Custom Resources

The operator introduces the following Custom Resource Definitions (CRDs):

### DynamicOperator

The DynamicOperator CRD allows you to define Kubernetes automation using Cyphernetes queries:

```yaml
apiVersion: cyphernetes.io/v1alpha1
kind: DynamicOperator
metadata:
  name: pod-cleaner
spec:
  query: |
    MATCH (p:Pod)
    WHERE p.status.phase = "Failed"
    DELETE p;
  schedule: "*/5 * * * *"  # Cron schedule
  dryRun: false  # Set to true to log actions without executing them
```

### QueryMonitor

The QueryMonitor CRD allows you to monitor cluster state using Cyphernetes queries:

```yaml
apiVersion: cyphernetes.io/v1alpha1
kind: QueryMonitor
metadata:
  name: deployment-monitor
spec:
  query: |
    MATCH (d:Deployment)
    WHERE d.status.availableReplicas < d.spec.replicas
    RETURN d.metadata.name, d.spec.replicas, d.status.availableReplicas;
  interval: 60s  # Check every 60 seconds
  alerting:
    slack:
      webhook: "https://hooks.slack.com/services/..."
```

## Common Use Cases

### Automated Cleanup

Clean up resources based on conditions:

```yaml
apiVersion: cyphernetes.io/v1alpha1
kind: DynamicOperator
metadata:
  name: cleanup-operator
spec:
  query: |
    // Delete completed jobs older than 24 hours
    MATCH (j:Job)
    WHERE j.status.completionTime < datetime() - duration("P1D")
    DELETE j;
  schedule: "0 * * * *"  # Run hourly
```

### Resource Validation

Enforce cluster policies:

```yaml
apiVersion: cyphernetes.io/v1alpha1
kind: QueryMonitor
metadata:
  name: resource-validator
spec:
  query: |
    // Find pods without resource limits
    MATCH (p:Pod)
    WHERE NOT EXISTS(p.spec.containers[0].resources.limits)
    RETURN p.metadata.name, p.metadata.namespace;
  interval: 300s
  alerting:
    email:
      to: "platform-team@company.com"
```

### Service Health Monitoring

Monitor service health across the cluster:

```yaml
apiVersion: cyphernetes.io/v1alpha1
kind: QueryMonitor
metadata:
  name: service-health
spec:
  query: |
    // Find services with no healthy endpoints
    MATCH (s:Service)-[:HAS_ENDPOINT]->(e:Endpoints)
    WHERE size(e.subsets) = 0
    RETURN s.metadata.name, s.metadata.namespace;
  interval: 30s
  alerting:
    slack:
      channel: "#platform-alerts"
```

## Configuration

### Operator Settings

The operator can be configured using environment variables or command-line flags:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cyphernetes-operator
spec:
  template:
    spec:
      containers:
      - name: manager
        args:
        - --metrics-bind-address=:8080
        - --health-probe-bind-address=:8081
        - --leader-elect=true
        env:
        - name: WATCH_NAMESPACE
          value: ""  # Watch all namespaces
```

### RBAC Configuration

The operator requires specific RBAC permissions to function:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cyphernetes-operator-role
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]
```

## Monitoring

### Operator Metrics

The operator exposes Prometheus metrics at `/metrics`:

- `cyphernetes_operator_reconciliations_total`
- `cyphernetes_operator_query_duration_seconds`
- `cyphernetes_operator_errors_total`

### Health Checks

Health and readiness probes are available at:
- Liveness: `/healthz`
- Readiness: `/readyz`

## Best Practices

1. **Start with DryRun**: When creating new DynamicOperators, start with `dryRun: true`
2. **Use Namespaces**: Scope operators to specific namespaces when possible
3. **Resource Limits**: Set appropriate resource limits for the operator
4. **Monitor Logs**: Keep track of operator logs for debugging
5. **Version Control**: Maintain operator configurations in version control

## Troubleshooting

Common issues and solutions:

1. **Permission Errors**
   - Verify RBAC configurations
   - Check operator service account permissions

2. **Query Timeouts**
   - Optimize complex queries
   - Adjust timeout settings

3. **Resource Constraints**
   - Monitor operator resource usage
   - Adjust resource limits 