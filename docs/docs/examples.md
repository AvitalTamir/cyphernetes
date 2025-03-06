---
sidebar_position: 6
---

# Examples

This guide provides practical examples of using Cyphernetes in various scenarios. Each example includes explanations and variations to help you understand how to adapt them to your needs.

## Basic Patterns

### Node Patterns

Basic node patterns with and without variables:

```graphql
// Basic node pattern
MATCH (p:Pod)
RETURN p;

// Node with properties
MATCH (d:Deployment {metadata: {name: "nginx"}})
RETURN d;

// Anonymous nodes
MATCH (p:Pod)->(:Service)->(e:Endpoints)
RETURN p, e;

// Kindless nodes (without specified resource type)
MATCH (d:Deployment {metadata: {name: "nginx"}})->(x)
RETURN p, x.kind;
```

### Resource Relationships

Different ways to express relationships between resources:

```graphql
// Right direction relationship
MATCH (p:Pod)->(s:Service)
RETURN p.metadata.name, s.metadata.name;

// Left direction relationship
MATCH (s:Service)<-(p:Pod)
RETURN p.metadata.name, s.metadata.name;

// Multiple relationships
MATCH (d:Deployment)->(rs:ReplicaSet)->(p:Pod)
RETURN d.metadata.name, rs.metadata.name, p.metadata.name;

// Anonymous relationships
MATCH (d:Deployment)->()->(p:Pod)
RETURN d.metadata.name, p.metadata.name;
```

## Resource Management

### Pod Management

Find and manage pods in your cluster:

```graphql
// List all pods that aren't running
MATCH (p:Pod)
WHERE p.status.phase != "Running"
RETURN p.metadata.name, p.status.phase;

// Find pods with no node assigned
MATCH (p:Pod)
WHERE p.spec.nodeName = NULL
RETURN p.metadata.name;

// Find pods with specific labels (with escaped dots)
MATCH (p:Pod)
WHERE p.metadata.labels.\kubernetes\.io/name = "nginx"
RETURN p.metadata.name;

// Find pods with high restart counts
MATCH (p:Pod)
WHERE p.status.containerStatuses[0].restartCount > 5
RETURN p.metadata.name, p.status.containerStatuses[0].restartCount;

// Delete pods that were created more than 7 days ago
MATCH (p:Pod)
WHERE p.metadata.creationTimestamp < datetime() - duration("P7D")
DELETE p;
```

### Deployment Management

Work with deployments and their related resources:

```graphql
// Scale deployments in a namespace
MATCH (d:Deployment {namespace: "production"})
SET d.spec.replicas = 3;

// Find deployments with mismatched replicas
MATCH (d:Deployment)
WHERE d.status.availableReplicas < d.spec.replicas
RETURN d.metadata.name, d.spec.replicas, d.status.availableReplicas;

// List pods for a specific deployment
MATCH (d:Deployment {app: "my-app"})->(:ReplicaSet)->(p:Pod)
RETURN p.metadata.name, p.status.phase;

// Update container images
MATCH (d:Deployment {app: "my-app"})
SET d.spec.template.spec.containers[0].image = "nginx:latest"
RETURN d.metadata.name;
```

## Service Discovery

### Service and Endpoint Analysis

Analyze services and their endpoints:

```graphql
// Find services without endpoints
MATCH (s:Service)
WHERE NOT (s)->(:core.Endpoints)
RETURN s.metadata.name;

// Find services with specific labels
MATCH (s:Service)
WHERE s.metadata.labels.app = "frontend"
RETURN s.metadata.name;

// Find services in multiple contexts
IN production, staging
MATCH (s:Service)
RETURN s.metadata.name, s.spec.clusterIP;
```

## Resource Relationships

### Cross-Resource Queries

Find relationships between different resource types:

```graphql
// Find all resources related to a deployment
MATCH (d:Deployment {app: "my-app"})->(x)
RETURN d, x.kind, x.metadata.name;

// Find configmaps used by pods
MATCH (p:Pod)->(c:ConfigMap)
RETURN p.metadata.name, c.metadata.name;

// Complex relationship chains
MATCH (d:Deployment {app: "my-app"})->(rs:ReplicaSet)->(p:Pod)->(s:Service)->(i:Ingress)
RETURN d.metadata.name, rs.metadata.name, p.metadata.name, s.metadata.name, i.metadata.name;
```