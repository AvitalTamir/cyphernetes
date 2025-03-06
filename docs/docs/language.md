---
sidebar_position: 3
---

# Language

Cyphernetes uses a Cypher-inspired query language that makes it intuitive to work with Kubernetes resources as a graph. This guide will walk you through the basics of the query language.

## Basic Concepts

In Cyphernetes, Kubernetes resources are represented as nodes in a graph. Each resource type (Pod, Service, Deployment, etc.) becomes a label, and the resource's metadata and spec become properties of the node. Resource types can be specified with their fully qualified names (e.g., `deployments.apps`, `core.pods`) and are case-insensitive (e.g., `Pod`, `POD`, `pod` are equivalent).

## Node Structure

A node is a representation of a Kubernetes resource. It has a kind, metadata, and spec.

```graphql
MATCH (n:Pod)
RETURN n;
```

Nodes may contain properties contained in curly braces. Node propeties may be `name`, `namespace` or any of the node's labels.

```graphql
MATCH (p:Pod {name: "my-pod", namespace: "my-namespace", app: "my-app"})
RETURN p;
```

## Query Structure

A typical Cyphernetes query consists of several clauses:

```graphql
IN (contexts)
MATCH (pattern)
WHERE (condition)
RETURN (expression)
```

## IN Clause (Optional)

The IN clause specifies which Kubernetes contexts to query. Context names can contain dashes, which is common in Kubernetes:

```graphql
// Query specific contexts
IN context1, my-cluster-name, prod-cluster
MATCH (p:Pod)
RETURN p.metadata.name;
```

## MATCH Clause

The MATCH clause is used to find patterns in your Kubernetes cluster:

```graphql
// Find all pods
MATCH (p:Pod)
RETURN p;

// Find pods and the services exposing them
MATCH (p:Pod)->(s:Service)
RETURN p, s;

// Relationship direction does not matter
MATCH (s:Service)<-(p:Pod)
RETURN p, s;

// Find resources through multiple relationships
MATCH (d:Deployment)->(rs:ReplicaSet)->(p:Pod)
RETURN d.metadata.name, p.metadata.name;

// Using "kindless" nodes allows you to match any resource kind
MATCH (d:Deployment)->(x)->(:Pod)
RETURN d;
```

Nodes can be anonymous (without a variable name) or named. Anonymous nodes are automatically assigned names:

```graphql
// Using anonymous nodes
MATCH (d:Deployment)->(:ReplicaSet)->(:Pod)
RETURN d;
```

Nodes can be both kindless and anonymous:

```graphql
MATCH (d:Deployment)->()->(:Pod)
RETURN d, p;
```

## WHERE Clause

The WHERE clause filters the results based on conditions. It supports both value comparisons and pattern matching:

```graphql
// Find pods that aren't running
MATCH (p:Pod)
WHERE p.status.phase != "Running"
RETURN p;

// Find pods in a specific namespace
MATCH (p:Pod)
WHERE p.metadata.namespace = "default"
RETURN p;

// Find pods with a specific label (with escaped dots)
MATCH (p:Pod)
WHERE p.metadata.labels.\kubernetes\.io/name = "nginx"
RETURN p;

// Find pods with no node assigned
MATCH (p:Pod)
WHERE p.spec.nodeName = NULL
RETURN p;
```

### Pattern Matching in WHERE Clause

```graphql
// Find services with no endpoints
MATCH (s:Service)
WHERE NOT (s)->(:Endpoints)
RETURN s.metadata.name;

// Find unused configmaps
MATCH (cm:ConfigMap)
WHERE NOT (cm)->(:Pod)
RETURN cm.metadata.name;
```

### Temporal Expressions

Cyphernetes supports temporal expressions for filtering resources based on their creation or modification times.
The `datetime()` function returns the current date and time in ISO 8601 format when called without arguments.
The `duration()` function returns a duration in ISO 8601 format.
You may use plus (+) and minus (-) operators to add or subtract durations from a datetime:

```graphql
// Find pods that were created in the last 24 hours
MATCH (p:Pod)
WHERE p.metadata.creationTimestamp > datetime() - duration("PT24H")
RETURN p.metadata.name;

// Delete pods that were created more than 7 days ago
MATCH (p:Pod)
WHERE p.metadata.creationTimestamp < datetime() - duration("P7D")
DELETE p;
```

### Supported operators:
- `=` (equals)
- `!=` (not equals)
- `>` (greater than)
- `<` (less than)
- `>=` (greater than or equals)
- `<=` (less than or equals)
- `=~` (regex match)
- `CONTAINS`

Values can be strings, numbers, booleans, or `NULL`.

Multiple conditions can be combined using `AND`:

```graphql
// Find unused persistent volume claims
MATCH (pvc:PersistentVolumeClaim)
WHERE NOT (pvc)->(:Pod) AND pvc.status.phase = "Bound"
RETURN pvc.metadata.name;
```

## RETURN Clause

The RETURN clause specifies what to return from the query. It supports JSON path expressions (including array access) and aggregation functions:

```graphql
// Return pod names
MATCH (p:Pod)
RETURN p.metadata.name;

// Return pod name and first container's image
MATCH (p:Pod)
RETURN p.metadata.name, p.spec.containers[0].image;

// Return all container images
MATCH (p:Pod)
RETURN p.spec.containers[*].image;

// Return nested array wildcards
MATCH (p:Pod)
RETURN p.spec.containers[*].volumeMounts[*].name;

// Using aggregation functions
MATCH (p:Pod)
RETURN COUNT{p} AS podCount;

// Sum container CPU requests
MATCH (d:Deployment)->(p:Pod)
RETURN SUM { p.spec.containers[0].resources.requests.cpu } AS totalCPUReq;
```

Supported aggregation functions:
- `COUNT`
- `SUM`

## Modifying Resources

### CREATE

Create new resources:

> Note: use proper JSON keys in CREATE statements, must be surrounded by double quotes.

```graphql
CREATE (n:Namespace {
  "metadata": {
    "name": "my-namespace"
  }
});
```

### SET

Modify resource properties:

```graphql
MATCH (d:Deployment {app: "my-app"})
SET d.spec.replicas = 3;
```

### DELETE

Delete resources:

```graphql
MATCH (p:Pod)
WHERE p.status.phase = "Failed"
DELETE p;
```

## Best Practices

1. **Use Specific Patterns**: Be as specific as possible in your MATCH patterns to improve query performance.
2. **Test Queries**: Test queries that modify resources in a non-production environment first.
3. **Use Variables**: Name your nodes when you need to reference them in WHERE or RETURN clauses.
4. **Context Awareness**: Use the IN clause when working with multiple clusters.

## Common Patterns

Here are some common query patterns:

### Finding Resources by Label

```graphql
MATCH (p:Pod {app: "my-app"})
RETURN p;
```

This is equivalent to:

```graphql
MATCH (p:Pod)
WHERE p.metadata.labels.app = "my-app"
RETURN p;
```

### Resource Relationships

```graphql
// Find pods belonging to a deployment
MATCH (d:Deployment {app: "my-app"})->(p:Pod)
RETURN d, p;

// Find services and their endpoints
MATCH (s:Service)->(e:Endpoints)
RETURN s, e;

// Find deployments through multiple relationships
MATCH (d:Deployment)->(rs:ReplicaSet)->(p:Pod)
RETURN d.metadata.name, rs.metadata.name, p.metadata.name;
```

### Complex Queries

```graphql
// Find pods that are not running and their associated services
MATCH (p:Pod)->(s:Service)
WHERE p.status.phase != "Running"
RETURN p.metadata.name, p.status.phase, s.metadata.name;

// Find deployments with less than desired replicas
MATCH (d:Deployment)
WHERE d.status.availableReplicas < d.spec.replicas
RETURN d.metadata.name, d.spec.replicas, d.status.availableReplicas;

// Find pods with specific container image patterns
MATCH (p:Pod)
WHERE p.spec.containers[*].image =~ "^nginx:.*"
RETURN p.metadata.name;

// Find pods with specific volume mount names
MATCH (p:Pod)
WHERE p.spec.containers[*].volumeMounts[*].name = "config"
RETURN p.metadata.name;
``` 