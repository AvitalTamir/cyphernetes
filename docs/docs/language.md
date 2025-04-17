---
sidebar_position: 1
---

# Language

Hi, welcome to Cyphernetes.

Let's get you started, you'll be querying the Kubernetes resource graph like a pro in no time!

----

## Node Patterns

In Cyphernetes, we draw patterns of resources using ASCII-art, using parentheses to denote nodes and arrows to denote relationships.

Let's draw a circle using parenthesis:

```graphql
()
```

This is called a _node_. Nodes are the basic building blocks of the Kubernetes resource graph.

Imagine a flow diagram, where each node represents a Kubernetes resource, and the edges (arrows) between them represent the relationships between the resources:

```graphql
()->()
```

This is Cyphernetes in a nutshell. You draw a diagram of the resources you want to work with, and Cyphernetes will translate it into the appropriate Kubernetes API calls.

Nodes are usually not empty. They tend to look more like this:

```graphql
(p:Pod)
```

This node contains a _variable_ (in this example it's called `p`), followed by a colon, followed by a _label_ (in our case, it's `Pod`).

We assign variable names to nodes so we can refer to them later in the query. Labels are used to specify the node's Kubernetes resource kind.

When specifying the label we can use the resource's singular name, plural name or shortname, just like in kubectl.
Unlike kubectl, labels in Cyphernetes are case-insensitive, so `(p:Pod)`, `(p:POD)`, `(p:pod)`, `(p:pods)`, `(p:po)` etc. are all legal and mean the same.

This document adheres to a convention of using minified, lowercase variable names and CamelCase, singular-name labels i.e. `(d:Deployment)`, `(rs:ReplicaSet)` - however this is completely up to the user.

Variable names are allowed to be mixed-case and have any length. Labels may also be mixed-case.
Unlike labels, variable names are case-sensitive, so `(d:Deployment)` and `(D:Deployment)` are not the same.

## Reading Resources from the Graph

To query the Kubernetes resource graph, we use `MATCH`/`RETURN` expressions.
`MATCH` is used to "draw" a pattern of resources, and will select all instances that match the pattern.

For example, `MATCH (d:Deployment)->(s:Service)` will ONLY select Deployments that are connected to a Service - i.e. exposed Deployments. It will not select any other Deployments that don't have a Service exposing them, or any Services that have selectors that don't match any Deployment.

This is a key feature of Cypher (and Cyphernetes) that is important to understand: **We act on patterns** - and only select resources that exactly match the pattern we draw.

`RETURN` is then used to organize the results. It takes a list of comma-separated JSONPaths, and returns the results in a JSON object, allowing us to easily craft a custom payload that only contains the fields we need.
Note that the names of resources are always returned in the `name` field, even when not specified in the `RETURN` clause.

```graphql
// Comments are supported
MATCH (d:Deployment) RETURN d.spec.replicas
```

This query will match all Deployments in the current context, and return their names:

```json
{
  "d": [
    {
      "name": "nginx",
      "spec": {
        "replicas": 4
      },
    },
    {
      "name": "nginx-internal",
      "spec": {
        "replicas": 2
      },
    }
  ]
}
```
Let's do one more:

```graphql
MATCH (d:Deployment)
RETURN d.metadata.labels,
       d.spec.replicas
```

This query will match all Deployments in the current context, and return a custom payload containing the fields we asked for:


```json
{
  "d": [
    {
      "metadata": {
        "labels": {
          "app": "nginx",
        },
      },
      "name": "nginx",
      "spec": {
        "replicas": 4
      }
    },
    {
      "metadata": {
        "labels": {
          "app": "nginx",
        },
      },
      "name": "nginx-internal",
      "spec": {
        "replicas": 2
      }
    }
  ]
}
```

The returned payload is a JSON object that contains a key for every variable defined in the `RETURN` clause.
Each of these keys' value is an array of Kubernetes resources that matched the respective node pattern in the `MATCH` clause. Unlike kubectl, Cyphernetes will **always return an array**, even if only one or zero resources were matched.

The payload will only include the fields requested in the `RETURN` clause. If only the variable name is specified in the `RETURN` clause, the payload will include the entire Kubernetes resource.

## Context

> Some Cyphernetes programs will allow you to change the default namespace or context, but this is beyond the scope of this document, which is focused on the Cyphernetes query language itself.

By default, Cyphernetes will query the current context (as defined by `kubectl config current-context`).
If no namespace is specified in the current context, Cyphernetes will default to using the `default` namespace, similar to kubectl.

### Overriding the Default Namespace

You can override the default namespace per node by specifying the `namespace` property in the node's properties:

```graphql
MATCH (d:Deployment {namespace: "staging"})->(s:Service)
RETURN d.metadata.name, s.spec.clusterIP
```

You can use this language feature to query resources across namespaces:

```graphql
MATCH (d:Deployment {namespace: "staging"}), (d2:Deployment {namespace: "production"})
RETURN d.spec.replicas, d2.spec.replicas
```

### Querying Multiple Clusters

Cyphernetes supports querying multiple clusters using the `IN` keyword.

```graphql
IN staging, production
MATCH (d:Deployment {name: "coredns", namespace: "kube-system"})
RETURN d.spec.replicas
```

Cyphernetes will run the query for each context in the `IN` clause, and return the results in a single payload.
The results will be prefixed with the context name, followed by an underscore:

```json
{
  "staging_d": [
    {
      "name": "coredns",
      "spec": {
        "replicas": 2
      }
    }
  ],
  "production_d": [
    {
      "name": "coredns",
      "spec": {
        "replicas": 2
      }
    }
  ]
}
```

## Advanced Pattern Matching

### Match by Name and Labels

A node may contain an optional set of properties. Node properties let us query the resource by name or by any of it's labels.

```graphql
MATCH (d:Deployment {name: "nginx-internal"})
RETURN d.metadata.labels,
       d.spec.template.spec.containers[0].image
```

(output)

```json
{
  "d": [
    {
      "metadata": {
        "labels": {
          "app": "nginx",
        },
      },
      "name": "nginx-internal",
      "spec": {
        "template": {
          "spec": {
            "containers[0]": {
              "image": "nginx"
            }
          }
        }
      }
    }
  ]
}
```

### Match by Any Field

Using the `WHERE` clause, we can filter our results by any field in the Kubernetes resource:

```graphql
MATCH (d:Deployment {app: "nginx", namespace: "default"})
WHERE d.spec.replicas=4
RETURN d.spec.replicas
```

(output)

```json
{
  "d": [
    {
      "name": "nginx",
      "spec": {
        "replicas": 4
      }
    }
  ]
}
```

### Escaping Dots in JSONPaths

Cyphernetes supports escaping dots in JSONPaths using a backslash. This is useful when querying resources that have dots in their field names.

```graphql
MATCH (d:Deployment {name: "nginx-internal"})
WHERE d.metadata.annotations.meta\.cyphernet\.es/foo-bar = "baz"
RETURN d.metadata.annotations.meta\.cyphernet\.es/foo-bar
```

(output)

```json
{
  "d": [
    {
      "name": "nginx-internal",
      "metadata": {
        "annotations": {
          "meta.cyphernet.es/foo-bar": "baz"
        }
      }
    }
  ]
}
```

`WHERE` clauses support the following operators:
* `=` - equal to
* `!=` - not equal to
* `<` - less than
* `>` - greater than
* `<=` - less than or equal to
* `>=` - greater than or equal to
* `=~` - regex matching
* `CONTAINS` - partial string matching

Examples:
```graphql
// Get all deployments with more than 2 replicas
MATCH (d:Deployment)
WHERE d.spec.replicas > 2
RETURN d.metadata.name, d.spec.replicas
```

```graphql
// Get all pods that are not running
MATCH (p:Pod)
WHERE p.status.phase != "Running"
RETURN p.metadata.name, p.status.phase
```

```graphql
// Find all deployments scaled above zero and set their related ingresses' ingressClassName to "active"
MATCH (d:Deployment)->(s:Service)->(i:Ingress)
WHERE d.spec.replicas >= 1
SET i.spec.ingressClassName = "active"
```

```graphql
// Find all deployments that end with "api"
MATCH (d:Deployment)
WHERE d.metadata.name =~ "^.*api$"
RETURN d.spec
```

### Matching Multiple Nodes

Use commas to match two or more nodes:

```graphql
MATCH (d:Deployment), (s:Service)
RETURN d.spec.replicas, s.spec.clusterIP
```

(output)

```json
{
  "d": [
    {
      "name": "nginx",
      "spec": {
        "replicas": 4
      }
    },
    {
      "name": "nginx-internal",
      "spec": {
        "replicas": 2
      }
    }
  ],
  "s": [
    {
      "name": "nginx",
      "spec": {
        "clusterIP": "10.96.0.1"
      }
    },
    {
      "name": "nginx-internal",
      "spec": {
        "clusterIP": "10.96.0.2"
      }
    }
  ]
}
```

## Relationships

Relationships are the glue that holds the Kubernetes resource graph together. Cyphernetes understands the relationships between Kubernetes resources, and lets us query them in a natural way.

Relationships are expressed using the `->` and `<-` operators:

```graphql
MATCH (d:Deployment)->(s:Service)
RETURN d.metadata.service, s.metadata.name
```

This query returns all Services that expose a Deployment, and the name of the Deployment they expose. Only Deployments and Services that have a relationship between them will be returned.

The relationship's direction is unimportant. `(d:Deployment)->(s:Service)` is the same as `(d:Deployment)<-(s:Service)`.

> If you're familiar with Cypher, you might be wondering about relationship properties. At this time, Cyphernetes does not make use of relationship properties - they are, however, legal - and you may use them if you wish for your own documentation purposes. i.e. `(d:Deployment)->[r:SERVICE_EXPOSE_DEPLOYMENT {"service-type": "kubernetes-internal"}]->(s:Service)` is legal Cyphernetes syntax, but does not affect the query's outcome. The variable `r` is not defined in this query, and is not available for use in a `RETURN` clause or otherwise.

### Basic Relationship Match

Cyphernetes understands the relationships between Kubernetes resources:

```graphql
MATCH (d:Deployment {name: "nginx"})->(s:Service)
RETURN s.spec.ports
```

(output)

```json
{
  "s": [
    {
      "name": "nginx",
      "spec": {
        "ports": [
          {
            "port": 80,
            "protocol": "TCP",
            "targetPort": 80
          }
        ]
      }
    }
  ]
}
```

Cyphernetes knows how to find related resources using a set of predefined rules. For example, Cyphernetes knows that a Service exposes a Deployment if the two resources have matching selectors.
Similarly, Cyphernetes knows that a Deployment owns a ReplicaSet if the ReplicaSet's `metadata.ownerReferences` contains a reference to the Deployment.

### Relationships with Multiple Nodes

We can match multiple nodes and relationships in a single MATCH clause. This is useful for working with resources that have multiple owners or with custom resources that Cyphernetes doesn't yet understand.

```graphql
MATCH (vs:VirtualService),
      (d:Deployment {name: "my-app"})->(s:Service)->(i:Ingress)
WHERE vs.metadata.labels.app="my-app"
RETURN i.spec.rules,
       vs.spec.http.paths
```

(output)

```json
{
  "i": [
    {
      "name": "my-app",
      "spec": {
        "rules": [
          {
            "host": "my-app.example.com",
            "http": {
              "paths": [
                {
                  "backend": {
                    "serviceName": "my-app",
                    "servicePort": 80
                  },
                  "path": "/"
                }
              ]
            }
          }
        ]
      }
    }
  ]
  "vs": [
    {
      "name": "my-app",
      "spec": {
        "http": {
          "paths": [
            {
              "backend": {
                "serviceName": "my-app",
                "servicePort": 80
              },
              "path": "/"
            }
          ]
        }
      }
    }
  ] 
}
```

> Here we match a Deployment, the Service that exposes it, and through the Service also the Ingress that routes to it. We also match the Istio VirtualService that belongs to the same application. Cyphernetes doesn't yet understand Istio, so we fallback to using the app label.

### Kindless Nodes

Sometimes you might want to match or operate on resources connected to another resource without knowing their kind in advance. Cyphernetes supports this through "kindless nodes" - nodes where you omit the kind label:

```graphql
// Find all resources related to a deployment
MATCH (d:Deployment {name: "nginx"})->(x)
RETURN x.kind
```

This query will find and return all resources that have a relationship with the "nginx" deployment, such as ReplicaSets and Services. Cyphernetes will automatically expand this query to try all possible kinds that can have a relationship with a Deployment.

> Some things to consider when using kindless nodes:
> * While kindless nodes are a powerful feature, they should be used judiciously. Being explicit about the kinds of resources you're operating on makes queries more predictable and easier to understand.
> * Chaining two kindless nodes (e.g., `MATCH (x)->(y)`) is not supported as it would be ambiguous and potentially expensive to resolve. At least one node in a relationship must have a known kind.
> * Standalone kindless nodes (e.g., `MATCH (x)`) are not supported. Kindless nodes must be part of a relationship.

### Anonymous Nodes

Anonymous nodes are nodes without a variable name. They are useful when you want to express a relationship path but don't want to use the intermediate resources in a subsequent `RETURN`, `SET` or `DELETE` clause.

```graphql
// Find all pods that are two relationships away from a deployment
MATCH (cm:ConfigMap)->(:Pod)
RETURN cm.data
```

For even more flexibility, you can use nodes that are both kindless and anonymous - nodes without both a variable name and kind:

```graphql
// Find all pods that are two relationships away from a deployment
MATCH (d:Deployment {name: "nginx"})->()->(p:Pod)
RETURN p.metadata.name
```

Kindless anonymous nodes are useful when you want to express a relationship path but don't care about the intermediate resources.

## Mutating the Graph

Cyphernetes supports creating, updating and deleting resources in the graph using the `CREATE`, `SET` and `DELETE` keywords.

### Creating Resources

Cyphernetes supports creating resources using the `CREATE` statement.
Currently, properties of nodes in `CREATE` clauses must be valid JSON. This is a temporary limitation that will be removed in the future.

```graphql
CREATE (k:Kind {"k": "v", "k2": "v2", ...})
```

### Creating a Standalone Resource

```graphql
CREATE (d:Deployment {
  "name": "nginx",
  "metadata": {
    "labels": {
      "app": "nginx"
    }
  },
  "spec": {
    "replicas": 4,
    "selector": {
      "matchLabels": {
        "app": "nginx"
      }
    },
    "template": {
      "metadata": {
        "labels": {
          "app": "nginx"
        }
      },
      "spec": {
        "containers": [
          {"name": "nginx", "image": "nginx"}
        ]
      }
    }
  }
})
```

Create expressions may optionally be followed by a `RETURN` clause:

```graphql
CREATE (d:Deployment {
  "name": "nginx",
  "metadata": {
    "labels": {
      "app": "nginx"
    }
  },
  "spec": {
    "replicas": 4,
    "selector": {
      "matchLabels": {
        "app": "nginx"
      }
    },
    "template": {
      "metadata": {
        "labels": {
          "app": "nginx"
        }
      },
      "spec": {
        "containers": [
          {"name": "nginx", "image": "nginx"}
        ]
      }
    }
  }
}) RETURN d
```

### Creating Resources by Relationship

Relationships can also appear in `CREATE` clauses. Currently, only two nodes may be connected by a relationship in a `CREATE` clause.

In `CREATE` clause relationships, one side of the relationship must contain a node variable that was previously defined in a `MATCH` clause.
This node does not require a label, as it's label is inferred from the `MATCH` clause.

The other side of the relationship is the new node being created.
Cyphernetes can infer the created resource's name, labels and other fields from the relationship rule defined between the two nodes' resource kinds.

```graphql
MATCH (d:Deployment {name: "nginx"})
CREATE (d)->(s:Service)
```

> This query is equivalent to `kubectl expose deployment nginx --type=ClusterIP`.

Cyphernetes' relationship rules contain a set default values for the created resource's fields. These defaults can be overridden by specifying properties in the `CREATE` clause. Default relationship fields should usually be enough for creating a resource by relationship without having to specify any properties on the created node.

### Patching Resources

Cyphernetes supports patching resources using the `SET` clause.

The `SET` clause is similar to the `CREATE` clause, but instead of creating a new resource, it updates an existing one. `SET` clauses take a list of comma-separated key-value pairs, where the key is a jsonPath to the field to update, and the value is the new value to set.

`SET` clauses may only appear after a `MATCH` clause. They may also be followed by a `RETURN` clause.

```graphql
MATCH (d:Deployment {name: "nginx"})
SET d.spec.replicas=4
RETURN d.spec.replicas
```

### Patch by Relationship

Relationships in `MATCH` clauses may be used to patch resources that are connected to other resources.

```graphql
MATCH (d:Deployment {name: "nginx"})->(s:Service)
SET s.spec.ports[0].port=8080
```

### Deleting Resources

Deleting resources is done using the `DELETE` clause. `DELETE` clauses may only appear after a `MATCH` clause.

A `DELETE` clause takes a list of variables from the `MATCH` clause - All Kubernetes resources matched by those variables will be deleted.

```graphql
MATCH (d:Deployment {name: "nginx"}) DELETE d
```

### Delete by Relationship

Relationships in `MATCH` clauses may be used to delete resources that are connected to other resources.

```graphql
MATCH (d:Deployment {name: "nginx"})->(s:Service)->(i:Ingress)
DELETE s, i
```

## Aggregations

Cyphernetes supports aggregations in the `RETURN` clause.
Currently, only the `COUNT` and `SUM` functions are supported.

```graphql
MATCH (d:Deployment)->(rs:ReplicaSet)->(p:Pod)
RETURN COUNT{p} AS TotalPods,
       SUM{d.spec.replicas} AS TotalReplicas

{
  ...
  "aggregate": {
    "TotalPods": 10,
    "TotalReplicas": 20
  },
  ...
}
```

```graphql
MATCH (d:deployment {name:"auth-service"})->(s:svc)->(p:pod) 
RETURN SUM { p.spec.containers[*].resources.requests.cpu } AS totalCPUReq, 
       SUM {p.spec.containers[*].resources.requests.memory } AS totalMemReq;


{
  ...
  "aggregate": {
    "totalCPUReq": "5",
    "totalMemReq": "336.0Mi"
  },
  ...
}
```

## Temporal Expressions

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