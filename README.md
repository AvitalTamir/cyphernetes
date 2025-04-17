<img width="1440" alt="Screenshot 2025-02-06 at 21 11 45" src="https://github.com/user-attachments/assets/d8eadc99-069e-4c95-aee6-b3530f890f03" />

![Cyphernetes Logo (3 5 x 1 2 in)](https://github.com/user-attachments/assets/2e0a92ce-26a6-4918-bc07-3747c2fe1464)
[![Go Report Card](https://goreportcard.com/badge/github.com/avitaltamir/cyphernetes)](https://goreportcard.com/report/github.com/avitaltamir/cyphernetes)
[![Go Reference](https://pkg.go.dev/badge/github.com/avitaltamir/cyphernetes.svg)](https://pkg.go.dev/github.com/avitaltamir/cyphernetes)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/cyphernetes-operator)](https://artifacthub.io/packages/search?repo=cyphernetes-operator)

Cyphernetes turns this: 😣
```bash
# Delete all pods that are not running

kubectl get pods --all-namespaces --field-selector 'status.phase!=Running' \
-o 'custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name' \
--no-headers | xargs -L1 -I {} bash -c 'set -- {}; kubectl delete pod $2 -n $1'
```

Into this: 🤩 
```cypher
// Do the same thing!

MATCH (p:Pod)
WHERE p.status.phase != "Running"
DELETE p;
```

📚 **Documentation**

For comprehensive documentation, visit our official documentation site at [https://cyphernet.es/docs](https://cyphernet.es).

## How?

Cyphernetes is a [Cypher](https://neo4j.com/developer/cypher/)-inspired query language for Kubernetes.
A mixture of ASCII-art, SQL and JSONPath, it lets the user express Kubernetes graph operations in a fun and creative way.
Cyphernetes works out-of-the-box with your CRDs, supports multi-cluster queries, and more.

There are multiple ways to run Cyphernetes queries:
1. Using the web client by running `cyphernetes web` from your terminal, then visiting `http://localhost:8080`
2. Using the interactive shell by running `cyphernetes shell` in your terminal
3. Running a single query from the command line by running `cyphernetes query "your query"` - great for scripting and CI/CD pipelines
4. Creating a [Cyphernetes DynamicOperator](https://github.com/avitaltamir/cyphernetes/blob/main/operator/test/e2e/samples/dynamicoperator-ingressactivator.yaml) using the cyphernetes-operator to define powerful Kubernetes workflows on-the-fly
5. Using the Cyphernetes API in your own Go programs.

### Examples
```cypher
// Get the desired and running replicas for all deployments
MATCH (d:Deployment)
RETURN d.spec.replicas AS desiredReplicas, 
       d.status.availableReplicas AS runningReplicas;

{
  "d": [
    {
      "desiredReplicas": 2,
      "name": "coredns",
      "runningReplicas": 2
    }
  ]
}

Query executed in 9.081292ms
```

Cyphernetes' superpower is understanding the relationships between Kubernetes resource kinds.
This feature is expressed using the arrows (`->`) you see in the example queries.
Relationships let us express connected operations in a natural way, and without having to worry about the underlying Kubernetes API:

```cypher
// This is similar to `kubectl expose`
MATCH (d:Deployment {name: "nginx"})
CREATE (d)->(s:Service);

Created services/nginx

Query executed in 30.692208ms
```

## Get Cyphernetes

Using Homebrew:

```bash
brew install cyphernetes
```

Using go:

```bash
go install github.com/avitaltamir/cyphernetes/cmd/cyphernetes@latest
```

Alternatively, grab a binary from the [Releases page](https://github.com/AvitalTamir/cyphernetes/releases).

## Development

The Cyphernetes monorepo is a multi-package project that includes the core Cyphernetes Go package, a CLI, a web client, and an operator.

```
.
├── cmd # The CLI (this is where the cyphernetes binary lives)
│   └── cyphernetes
│       └── ...
├── docs # The cyphernet.es website
│   └── ...
├── operator # The operator
│   └── ...
├── pkg
│   └── core # The core Cyphernetes package (parser and engine behind the language features)
│   └── provider # An interface for different backend implementations
│       └── apiserver # A client for the Kubernetes API server
├── web # The web client
│   └── src
│       └── ...
```

### Prerequisites

* Go (Latest)
* Make (for running make commands)
* NodeJS (Latest, for building the web client)
* pnpm (9+, for building the web client)

### Getting Started

To get started with development:

Clone the repository:

```bash
git clone https://github.com/avitaltamir/cyphernetes.git
```

Navigate to the project directory:

```bash
cd cyphernetes
```

### Building the Core Project

Running `make` will build the operator manifests and web client static assets, then build the binary and run the tests.

```bash
make
```

### Building the Web Client

```bash
make web-build
```

### Building the Operator

```bash
make operator-build
```

### Contributing

Contributions are welcome! Please feel free to submit pull requests, open issues, and provide feedback.

## License

Cyphernetes is open-sourced under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.

## Acknowledgments

* Thanks to [Neo4j](https://neo4j.com/) for the inspiration behind the query language.
* Thanks to [ggerganov](https://github.com/ggerganov) for the [dot-to-ascii](https://github.com/ggerganov/dot-to-ascii) project - it's the webserver that serves the ASCII art on [https://ascii.cyphernet.es](https://ascii.cyphernet.es) in case you want to host your own.
* Thanks to [shlomif](https://github.com/shlomif) for the [graph-easy](https://github.com/shlomif/graph-easy) project - it's the package that actually converts the dot graphs into ASCII art used by dot-to-ascii.
* Thanks [anthonybrice](https://github.com/anthonybrice) and [chenrui333](https://github.com/chenrui333) for getting us into Homebrew.

## Authors

* _Initial work_ - [Avital Tamir](https://github.com/avitaltamir)
* _Project maintainer_ - [Naor Peled](https://github.com/naorpeled)
* _Enhancements, Bug fixes_ - [James Kim](https://github.com/jameskim0987)
* _Enhancements, Bug fixes_ - [Vinicius Artur](https://github.com/viniciusartur)
* _Improvements_ - [Noé Tarbouriech](https://github.com/noetarbouriech)
