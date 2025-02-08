---
sidebar_position: 6
---

# Examples

This guide provides practical examples of using Cyphernetes in various scenarios. Each example includes explanations and variations to help you understand how to adapt them to your needs.

## Resource Management

### Pod Management

Find and manage pods in your cluster:

```graphql
// List all pods that aren't running
MATCH (p:Pod)
WHERE p.status.phase != "Running"
RETURN p.metadata.name, p.status.phase;

// Delete failed pods older than 1 hour
MATCH (p:Pod)
WHERE p.status.phase = "Failed"
  AND p.status.startTime < datetime() - duration("PT1H")
DELETE p;

// Find pods with high restart counts
MATCH (p:Pod)
WHERE p.status.containerStatuses[0].restartCount > 5
RETURN p.metadata.name, p.status.containerStatuses[0].restartCount;
```

### Deployment Management

Work with deployments and their related resources:

```graphql
// Scale all deployments in a namespace
MATCH (d:Deployment)
WHERE d.metadata.namespace = "production"
SET d.spec.replicas = 3;

// Find deployments with mismatched replicas
MATCH (d:Deployment)
WHERE d.status.availableReplicas != d.spec.replicas
RETURN d.metadata.name,
       d.spec.replicas as desired,
       d.status.availableReplicas as actual;

// List pods for a specific deployment
MATCH (d:Deployment {metadata: {name: "nginx"}})-[:CONTROLS]->(p:Pod)
RETURN p.metadata.name, p.status.phase;
```

## Service Discovery

### Service and Endpoint Analysis

Analyze services and their endpoints:

```graphql
// Find services without endpoints
MATCH (s:Service)
WHERE NOT (s)-[:HAS_ENDPOINT]->(:Endpoints)
RETURN s.metadata.name;

// List services and their endpoint counts
MATCH (s:Service)-[:HAS_ENDPOINT]->(e:Endpoints)
RETURN s.metadata.name,
       size(e.subsets[0].addresses) as endpoints;

// Find services with specific labels
MATCH (s:Service)
WHERE s.metadata.labels.app = "frontend"
RETURN s;
```

## Resource Relationships

### Cross-Resource Queries

Find relationships between different resource types:

```graphql
// Find ingresses and their backend services
MATCH (i:Ingress)-[:ROUTES_TO]->(s:Service)
RETURN i.metadata.name as ingress,
       s.metadata.name as service;

// Trace pod to node relationships
MATCH (p:Pod)-[:RUNS_ON]->(n:Node)
RETURN p.metadata.name as pod,
       n.metadata.name as node;

// Find configmaps used by pods
MATCH (p:Pod)-[:USES_CONFIG]->(c:ConfigMap)
RETURN p.metadata.name as pod,
       c.metadata.name as config;
```

## Monitoring and Debugging

### Resource State Analysis

Monitor and debug cluster state:

```graphql
// Find nodes with low disk space
MATCH (n:Node)
WHERE n.status.capacity.ephemeral_storage < n.status.allocatable.ephemeral_storage * 0.1
RETURN n.metadata.name,
       n.status.capacity.ephemeral_storage,
       n.status.allocatable.ephemeral_storage;

// Check pod resource requests vs limits
MATCH (p:Pod)
WHERE p.spec.containers[0].resources.requests.cpu != p.spec.containers[0].resources.limits.cpu
RETURN p.metadata.name,
       p.spec.containers[0].resources.requests.cpu as cpu_request,
       p.spec.containers[0].resources.limits.cpu as cpu_limit;
```

## Security and Compliance

### Security Checks

Perform security audits:

```graphql
// Find pods running as root
MATCH (p:Pod)
WHERE p.spec.containers[0].securityContext.runAsNonRoot != true
RETURN p.metadata.name;

// Check for privileged containers
MATCH (p:Pod)
WHERE p.spec.containers[0].securityContext.privileged = true
RETURN p.metadata.name,
       p.metadata.namespace;

// Find pods without network policies
MATCH (p:Pod)
WHERE NOT (p)<-[:APPLIES_TO]-(:NetworkPolicy)
RETURN p.metadata.name;
```

## Operator Examples

### DynamicOperator Use Cases

Example operator configurations:

```yaml
# Automatic Pod Cleanup
apiVersion: cyphernetes.io/v1alpha1
kind: DynamicOperator
metadata:
  name: pod-janitor
spec:
  query: |
    MATCH (p:Pod)
    WHERE p.status.phase in ["Failed", "Succeeded"]
      AND p.status.startTime < datetime() - duration("P7D")
    DELETE p;
  schedule: "0 0 * * *"  # Run daily at midnight

---
# Resource Quota Monitor
apiVersion: cyphernetes.io/v1alpha1
kind: QueryMonitor
metadata:
  name: quota-monitor
spec:
  query: |
    MATCH (n:Namespace)-[:HAS_QUOTA]->(q:ResourceQuota)
    WHERE q.status.used.cpu > q.spec.hard.cpu * 0.8
    RETURN n.metadata.name,
           q.status.used.cpu as used,
           q.spec.hard.cpu as limit;
  interval: 300s
  alerting:
    slack:
      channel: "#ops-alerts"
```

## Advanced Patterns

### Complex Queries

More sophisticated query patterns:

```graphql
// Find orphaned PVCs
MATCH (pvc:PersistentVolumeClaim)
WHERE NOT (pvc)<-[:USES_STORAGE]-(:Pod)
  AND pvc.status.phase = "Bound"
RETURN pvc.metadata.name;

// Analyze service mesh topology
MATCH path = (s1:Service)-[:COMMUNICATES_WITH*1..3]->(s2:Service)
WHERE s1.metadata.name = "frontend"
RETURN path;

// Resource usage by namespace
MATCH (n:Namespace)<-[:IN_NAMESPACE]-(p:Pod)
WITH n.metadata.name as namespace,
     sum(toFloat(p.spec.containers[0].resources.requests.cpu)) as total_cpu
RETURN namespace, total_cpu
ORDER BY total_cpu DESC;
```

## Best Practices

1. **Use Variables**: For complex patterns, use variables to make queries more readable
2. **Add Comments**: Document complex queries with inline comments
3. **Test First**: Test queries with RETURN before using DELETE or SET
4. **Use Indexes**: Consider resource labels and common properties in WHERE clauses
5. **Limit Results**: Use LIMIT for large result sets 