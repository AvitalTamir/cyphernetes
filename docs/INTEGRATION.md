# Integrating Cyphernetes in your own Go project

This guide will help you integrate Cyphernetes into your own Go project.
Cyphernetes is made up of two main packages:

1. The `pkg/core` package, which contains the Cyphernetes parser and engine.
2. The `pkg/provider` package, which contains the Cyphernetes provider interface and a default implementation for an api-server client.

## Integrating the `pkg/core` package

The `pkg/core` package is a library that you can import into your own Go project.
It provides a single function, `Parse`, which takes a Cyphernetes query and returns a Result object, which contains the results data and a graph made up of nodes and edges.

To use the `Parse` function, you need to import the `pkg/core` package and instantiate a new `QueryExecutor` using the `NewQueryExecutor` function - to which you pass a `Provider` implementation:

```go
import (
    "github.com/avitaltamir/cyphernetes/pkg/core"
    "github.com/avitaltamir/cyphernetes/pkg/provider"
)

provider := provider.NewAPIServerProvider()
executor := core.NewQueryExecutor(provider)
query := "MATCH (p:Pod) WHERE p.status.phase != 'Running' RETURN p.metadata.name"
result, err := executor.Parse(query)
if err != nil {
    log.Fatalf("Error parsing query: %v", err)
}
fmt.Printf("Result: %+v\n", result)
```

Out of the box, Cyphernetes ships with a default implementation for an api-server client, which is the `pkg/provider/apiserver` package. This package is a wrapper around the Kubernetes client-go library, and provides a `Provider` interface that you can implement in your own project - and use the Cyphernetes parser and engine with a different backend.

The provider interface is defined in the `pkg/provider/interface.go` file:

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

To implement the provider interface, you can use the `pkg/provider/apiserver` package as a reference implementation.
The 4 CRUD operations are pretty straightforward. They all take a kind, and namespace, "Get" operations take a fieldSelector and labelSelector, while "Create" and "Patch" operations take a body (JSON for "Create", and a JSON patch for "Patch").

The schema operation functions are as follows:

- `FindGVR` is used to find the GVR for a given kind. This is used by the Cyphernetes parser to find the correct API endpoint to query. It returns an `apimachinery/pkg/runtime/schema.GroupVersionResource`, which contains the Group, Version, and Resource for the given kind.
- `GetOpenAPIResourceSpecs` is used to get a flat list of JSONPaths for a given kind. This is used by the Cyphernetes parser to understand the API schema, which allows it to infer relationships between resources.
Example:
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
- `CreateProviderForContext` is used to create a new provider for a given context. This is used by the Cyphernetes engine when running multi-context queries only.

# Kubernetes Client

The `pkg/provider/apiserver` package is a wrapper around the Kubernetes client-go library and may be used as a base implementation for your own provider. If your program already uses client-go, you can re-use a lot of the code in the `pkg/provider/apiserver` package, and only implement the parts that are missing from your provider - for example retrieving the resources from a Postgres or Elasticsearch database.

Alternatively, you can implement your own provider from scratch, as long as it:
- implements the `Provider` interface
- `FindGVR` correctly resolves strings to `schema.GroupVersionResource` objects
- `GetOpenAPIResourceSpecs` provides a list of JSONPaths for each kind
- You may choose to make this a "read-only" provider by having CUD operations return an error or warning - or implement the full CRUD operations.
- You may choose to support multiple Kubernetes contexts, or leave out this functionality and return an error from `CreateProviderForContext` if the user tries to run a multi-context query.

