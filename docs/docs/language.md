---
sidebar_position: 3
---

# Language

Cyphernetes uses a Cypher-inspired query language that makes it intuitive to work with Kubernetes resources as a graph. This guide will walk you through the basics of the query language.

## Basic Concepts

In Cyphernetes, Kubernetes resources are represented as nodes in a graph. Each resource type (Pod, Service, Deployment, etc.) becomes a label, and the resource's metadata and spec become properties of the node.

## Query Structure

A typical Cyphernetes query consists of several clauses:

```graphql
MATCH (pattern)
WHERE (condition)
RETURN (expression)
```

## MATCH Clause

The MATCH clause is used to find patterns in your Kubernetes cluster. For example:

```graphql
// Find all pods
MATCH (p:Pod)
RETURN p;

// Find pods and their services
MATCH (p:Pod)-[:BELONGS_TO]->(s:Service)
RETURN p, s;
```

## WHERE Clause

The WHERE clause filters the results based on conditions:

```graphql
// Find pods that aren't running
MATCH (p:Pod)
WHERE p.status.phase != "Running"
RETURN p;

// Find pods in a specific namespace
MATCH (p:Pod)
WHERE p.metadata.namespace = "default"
RETURN p;
```

## RETURN Clause

The RETURN clause specifies what to return from the query:

```graphql
// Return pod names
MATCH (p:Pod)
RETURN p.metadata.name;

// Return pod name and status
MATCH (p:Pod)
RETURN p.metadata.name, p.status.phase;
```

## Modifying Resources

Cyphernetes also supports modifying resources:

### CREATE

Create new resources:

```graphql
// Create a new namespace
CREATE (n:Namespace {
  metadata: {
    name: "my-namespace"
  }
});
```

### DELETE

Delete resources:

```graphql
// Delete all failed pods
MATCH (p:Pod)
WHERE p.status.phase = "Failed"
DELETE p;
```

### SET

Modify resource properties:

```graphql
// Scale a deployment
MATCH (d:Deployment {metadata: {name: "my-app"}})
SET d.spec.replicas = 3;
```

## Common Patterns

Here are some common query patterns:

### Finding Resources by Label

```graphql
// Find pods with specific labels
MATCH (p:Pod)
WHERE p.metadata.labels.app = "my-app"
RETURN p;
```

### Resource Relationships

```graphql
// Find pods belonging to a deployment
MATCH (d:Deployment {metadata: {name: "my-app"}})-[:CONTROLS]->(p:Pod)
RETURN p;

// Find services and their endpoints
MATCH (s:Service)-[:HAS_ENDPOINT]->(e:Endpoints)
RETURN s, e;
```

### Complex Queries

```graphql
// Find pods that are not running and their associated services
MATCH (p:Pod)-[:BELONGS_TO]->(s:Service)
WHERE p.status.phase != "Running"
RETURN p.metadata.name, p.status.phase, s.metadata.name;

// Find deployments with less than desired replicas
MATCH (d:Deployment)
WHERE d.status.availableReplicas < d.spec.replicas
RETURN d.metadata.name, d.spec.replicas, d.status.availableReplicas;
```

## Best Practices

1. **Use Specific Patterns**: Be as specific as possible in your MATCH patterns to improve query performance.
2. **Limit Results**: Use LIMIT when you don't need all results.
3. **Use Parameters**: Use parameters for values that might change instead of hardcoding them.
4. **Test Queries**: Test queries that modify resources in a non-production environment first. 