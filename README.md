![Cyphernetes Logo (3 5 x 1 2 in)](https://github.com/user-attachments/assets/2e0a92ce-26a6-4918-bc07-3747c2fe1464)

[![Go Report Card](https://goreportcard.com/badge/github.com/avitaltamir/cyphernetes)](https://goreportcard.com/report/github.com/avitaltamir/cyphernetes)
[![Go Reference](https://pkg.go.dev/badge/github.com/avitaltamir/cyphernetes.svg)](https://pkg.go.dev/github.com/avitaltamir/cyphernetes)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Cyphernetes turns this: 😣
```bash
# Select all zero-scaled Deployments in all namespaces,
# find all Ingresses routing to these deployments -
# for each Ingress change it's ingress class to 'inactive':

kubectl get deployments -A -o json | jq -r '.items[] | select(.spec.replicas == 0) | \
[.metadata.namespace, .metadata.name, (.spec.selector | to_entries | map("\(.key)=\(.value)") | \
join(","))] | @tsv' | while read -r ns dep selector; do kubectl get services -n "$ns" -o json | \
jq -r --arg selector "$selector" '.items[] | select((.spec.selector | to_entries | \
map("\(.key)=\(.value)") | join(",")) == $selector) | .metadata.name' | \
while read -r svc; do kubectl get ingresses -n "$ns" -o json | jq -r --arg svc "$svc" '.items[] | \
select(.spec.rules[].http.paths[].backend.service.name == $svc) | .metadata.name' | \
xargs -I {} kubectl patch ingress {} -n "$ns" --type=json -p \
'[{"op": "replace", "path": "/spec/ingressClassName", "value": "inactive"}]'; done; done
```

Into this: 🤩 
```graphql
# Do the same thing!

MATCH (d:Deployment)->(s:Service)->(i:Ingress)
WHERE d.spec.replicas=0
SET i.spec.ingressClassName="inactive";
```

## How?

Cyphernetes is a [Cypher](https://neo4j.com/developer/cypher/)-inspired query language for Kubernetes.
It is a mixture of ASCII-art, SQL and JSON and it lets us express Kubernetes operations in an efficeint way that is also fun and creative.

There are multiple ways to run Cyphernetes queries:
1. Using the interactive shell by running `cyphernetes shell` in your terminal
2. Running a single query from the command line by running `cyphernetes query "your query"` - great for scripting and CI/CD pipelines
3. Creating a [Cyphernetes DynamicOperator](https://github.com/avitaltamir/cyphernetes/blob/main/operator/helm/cyphernetes-operator/samples/dynamicoperator-ingressactivator.yaml) using the cyphernetes-operator which lets you define powerful Kubernetes workflows on-the-fly
4. Using the Cyphernetes API in your own Go programs

To learn more about how to use Cyphernetes, refer to these documents:
* [LANGUAGE.md](docs/LANGUAGE.md) - a crash-course in Cyphernetes language syntax
* [CLI.md](docs/CLI.md) - a guide to using Cyphernetes shell, query command and macros
* [OPERATOR.md](docs/OPERATOR.md) - a guide to using Cyphernetes DynamicOperator

### Some examples from the shell
```graphql
# Get the desired and running replicas for all deployments
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

```graphql
# This is similar to `kubectl expose`
> MATCH (d:Deployment {name: "nginx"})
  CREATE (d)->(s:Service);

Created services/nginx

Query executed in 30.692208ms
```

### It has macros and graphs too
Macros are minimalistic, user-extensible & batteries included stored procedures.
They turn the Cyphernetes shell into a handy kubectl alternative.
Many useful macros are included - and it's easy to define your own.

```graphql
# This macro creates a service and public ingress for a deployment.
# It's defined like this:
# :exposepublic deploymentName hostname # Expose a deployment as a service and ingress
# MATCH (deployment:Deployment {name: "$deploymentName"})
# CREATE (deployment)->(service:Service);
# MATCH (services:Service {name: "$deploymentName"})
# CREATE (services)->(i:ingress {"spec":{"rules": [{"host": "$hostname"}]}});
# MATCH (deployments:Deployment {name: "$deploymentName"})->(services:Service)->(ingresses:Ingress)
# RETURN services.metadata.name, services.spec.type AS Type, services.spec.clusterIP AS ClusterIP, ingresses.spec.rules[0].host AS Host, ingresses.spec.rules[0].http.paths[0].path AS Path, ingresses.spec.rules[0].http.paths[0].backend.service.name AS Service;

# Cyphernetes can optionally draw a graph of affected nodes as ASCII-art!

> :expose_public nginx foo.com
Created services/nginx
Created ingresses/nginx

┌─────────────────┐
│ *Ingress* nginx │
└─────────────────┘
  │
  │ :ROUTE
  ▼
┌─────────────────┐
│ *Service* nginx │
└─────────────────┘

{
  "ingresses": [
    {
      "Host": "foo.com",
      "Path": "/",
      "Service": "nginx",
      "name": "nginx"
    }
  ],
  "services": [
    {
      "ClusterIP": "10.96.164.152",
      "Type": "ClusterIP",
      "name": "nginx"
    }
  ]
}

Macro executed in 50.305083ms
```

## Get Cyphernetes

Using go:

```bash
go install github.com/avitaltamir/cyphernetes/cmd/cyphernetes@latest
```

Alternatively, grab a binary from the [Releases page](https://github.com/AvitalTamir/cyphernetes/releases).

## Development

Cyphernetes is written in Go and utilizes a parser generated by goyacc to interpret the custom query language.

### Prerequisites

* Go (Latest)
* goyacc (for generating the parser)
* Make (for running make commands)

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

### Building the Project

Use the Makefile commands to build the project:

* Build & Test:

```bash
make
```

* To build the binary:

```bash
make build
```

* To run tests:

```bash
make test
```

* To generate the grammar parser:

```bash
make gen-parser
```

* To clean up the build:

```bash
make clean
```

### Contributing

Contributions are welcome! Please feel free to submit pull requests, open issues, and provide feedback.

## License

Cyphernetes is open-sourced under the MIT license. See the [LICENSE](LICENSE) file for details.

## Acknowledgments

* Thanks to [Neo4j](https://neo4j.com/) for the inspiration behind the query language.
* Thanks to [ggerganov](https://github.com/ggerganov) for the [dot-to-ascii](https://github.com/ggerganov/dot-to-ascii) project - it's the webserver that serves the ASCII art on [https://ascii.cyphernet.es](https://ascii.cyphernet.es) in case you want to host your own.
* Thanks to [shlomif](https://github.com/shlomif) for the [graph-easy](https://github.com/shlomif/graph-easy) project - it's the package that actually converts the dot graphs into ASCII art used by dot-to-ascii.

## Authors

* _Initial work_ - [Avital Tamir](https://github.com/avitaltamir)
