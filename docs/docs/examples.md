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

// Relationship direction doesn't matter
MATCH (p:Pod)<-(s:Service)
RETURN p.metadata.name, s.metadata.name;

// Chained relationships
MATCH (d:Deployment)->(rs:ReplicaSet)->(p:Pod)
RETURN d.metadata.name, rs.metadata.name, p.metadata.name;

// Anonymous, kindless relationships
MATCH (d:Deployment)->()->(p:Pod)
RETURN d.metadata.name, p.metadata.name;

// Find all resources related to a deployment
MATCH (d:Deployment {app: "my-app"})->(x)
RETURN d, x.kind, x.metadata.name;

// Complex relationship chains
MATCH (d:Deployment {app: "my-app"})->(rs:ReplicaSet)->(p:Pod)->(s:Service)->(i:Ingress)
RETURN d.metadata.name, rs.metadata.name, p.metadata.name, s.metadata.name, i.metadata.name;
```

## Resource Management

### Pod Management

Find and manage pods in your cluster:

```graphql
// Delete all pods that aren't running
MATCH (p:Pod)
WHERE p.status.phase != "Running"
DELETE p;

// Find pods with no node assigned
MATCH (p:Pod)
WHERE p.spec.nodeName = NULL
RETURN p.metadata.name;

// Find pods with specific labels (with escaped dots)
MATCH (p:Pod)
WHERE p.metadata.labels.kubernetes\.io/name = "nginx"
RETURN p.metadata.name;

// Find pods with high restart counts
MATCH (p:Pod)
WHERE p.status.containerStatuses[0].restartCount > 5
RETURN p.metadata.name, p.status.containerStatuses[0].restartCount;
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

### Cluster Maintenance

```graphql
// Find configmaps not used by any pod
MATCH (cm:ConfigMap)
WHERE NOT (cm)->(:Pod)
RETURN cm.metadata.name;

// Find orphaned PersistentVolumeClaims
MATCH (pvc:PersistentVolumeClaim)
WHERE NOT (pvc)->(:PersistentVolume)
AND pvc.status.phase != "Bound"
RETURN pvc.metadata.name;

// Delete pods that are not running and were created more than 7 days ago
MATCH (p:Pod)
WHERE p.status.phase != "Running"
AND p.metadata.creationTimestamp < datetime() - duration("P7D")
DELETE p;
```

### Service and Endpoint Analysis

```graphql
// Find services without endpoints
MATCH (s:Service)
WHERE NOT (s)->(:core.Endpoints)
RETURN s.metadata.name;

// Find services with specific labels
MATCH (s:Service {app: "frontend"})
RETURN s.metadata.name;

// Find services in multiple contexts
IN production, staging
MATCH (s:Service {name: "api"})
RETURN s.metadata.name, s.spec.clusterIP;
```