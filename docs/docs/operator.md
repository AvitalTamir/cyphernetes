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
helm pull oci://ghcr.io/avitaltamir/cyphernetes/cyphernetes-operator
tar -xvf cyphernetes-operator-*.tgz
cd cyphernetes-operator
helm upgrade --install cyphernetes-operator . --namespace cyphernetes-operator --create-namespace
```

## Custom Resources

The operator introduces the following Custom Resource Definition (CRD):

### DynamicOperator

The DynamicOperator CRD allows you to define Kubernetes automation using Cyphernetes queries. It watches specified resources and executes queries in response to resource events:

```yaml
apiVersion: cyphernetes-operator.cyphernet.es/v1
kind: DynamicOperator
metadata:
  name: pod-cleaner
  namespace: default
spec:
  resourceKind: pods
  namespace: default
  onUpdate: |
    MATCH (p:Pod {metadata: {name: "{{$.metadata.name}}"}})
    WHERE p.status.phase = "Failed"
    DELETE p;
```

The DynamicOperator spec supports the following fields:
- `resourceKind` (required): The Kubernetes resource kind to watch
- `namespace`: The namespace to watch (if empty, watches all namespaces)
- `onCreate`: Query to execute when a resource is created
- `onUpdate`: Query to execute when a resource is updated
- `onDelete`: Query to execute when a resource is deleted
- `finalizer`: Whether to register a finalizer on watched resources (defaults to false)

At least one of `onCreate`, `onUpdate`, or `onDelete` must be specified.

## Common Use Cases

### Automated Cleanup

Clean up resources based on conditions:

```yaml
apiVersion: cyphernetes-operator.cyphernet.es/v1
kind: DynamicOperator
metadata:
  name: cleanup-operator
  namespace: default
spec:
  resourceKind: jobs
  namespace: default
  onUpdate: |
    MATCH (j:Job {metadata: {name: "{{$.metadata.name}}"}})
    WHERE j.status.completionTime != NULL
      AND j.status.succeeded > 0
    DELETE j;
```

### Resource Validation

Monitor and enforce cluster policies:

```yaml
apiVersion: cyphernetes-operator.cyphernet.es/v1
kind: DynamicOperator
metadata:
  name: resource-validator
  namespace: default
spec:
  resourceKind: pods
  namespace: default
  onCreate: |
    MATCH (p:Pod {metadata: {name: "{{$.metadata.name}}"}})
    WHERE NOT EXISTS(p.spec.containers[0].resources.limits)
    DELETE p;
```

### Service Health Monitoring

Monitor service health across the cluster:

```yaml
apiVersion: cyphernetes-operator.cyphernet.es/v1
kind: DynamicOperator
metadata:
  name: service-health
  namespace: default
spec:
  resourceKind: services
  namespace: default
  onUpdate: |
    MATCH (s:Service {metadata: {name: "{{$.metadata.name}}"}})
    WHERE NOT (s)->(:core.Endpoints)
    DELETE s;
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

1. **Start with Dry Run**: Test your operators in a development environment first
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