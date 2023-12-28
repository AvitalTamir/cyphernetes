


https://github.com/AvitalTamir/cyphernetes/assets/83203533/22215470-6d84-452a-b390-ba38bd82bf17


<table style="border-collapse: collapse; border: none">
  <tr>
    <td style="border: none" width="256">
      <img src="./logo.png" alt="Cyphernetes Logo" width="256">
    </td>
    <td style="border: none; padding-left: 20px">
      <h1>Cyphernetes</h1>
      <p>Cyphernetes is a library (and CLI tool) designed to manage Kubernetes resources using a query language inspired by Cypher, the query language of Neo4j. It provides a more intuitive way to interact with Kubernetes clusters, allowing users to express complex operations as graph-like queries.</p>
    </td>
  </tr>
</table>

## Why Cyphernetes?

Kubernetes management often involves dealing with complex and verbose command-line instructions. Cyphernetes simplifies this complexity by introducing a declarative query language that can express these instructions in a more readable and concise form. By leveraging a query language similar to Cypher, users can efficiently perform CRUD operations on Kubernetes resources, visualize resource connections, and manage their Kubernetes clusters with greater ease and flexibility.

The project it still at an early stage. All basic CRUD functionality is there,
but much more testing and wiring between resource kinds is still left to do and the Cypher-like grammar implementation is incomplete.
A high-level list of what's still missing:

* WHERE and AS clauses
* The graph model of relationships between common Kubernetes resources is in very early stages

See the [project roadmap](https://github.com/AvitalTamir/cyphernetes/blob/main/ROADMAP.md) for more detailed information on what's still left to do.

## Install

```bash
# With "go install"
$ go install github.com/avitaltamir/cyphernetes/cmd/cyphernetes@latest
```

This is currently the only channel to install Cyphernetes, you'll need go installed for this.

Otherwise, clone this repo and run `make`.
You'll need to have go as well as `goyacc` in your path to be able to build.

## Usage

Cyphernetes offers two main commands: `query` and `shell`. Below are examples of how to use these commands with different types of queries.

### Query Command

The `query` command is used for running single Cyphernetes queries from the command line.

Example usage:

```bash
cyphernetes query 'MATCH (d:Deployment {name: "nginx"}) RETURN d'
```

This command retrieves information about a Deployment named 'nginx'.

### Shell Command

The shell command launches an interactive shell where you can execute multiple queries in a session.

To start the shell:

```bash
cyphernetes shell
```

Type 'exit' or press Ctrl-C/Ctrl-D to leave the shell.

## Query Examples

### Basic Node Match

```graphql
MATCH (d:Deployment) RETURN d
```

This query lists all Deployment resources.

### Node with Properties

```graphql
MATCH (d:Deployment {app: "nginx"}) RETURN d
```

Retrieves Deployments where the app label is 'nginx'.

### Multiple Nodes

```graphql
# Multiple matches
MATCH (d:Deployment), (s:Service), (i:Ingress) RETURN d, s, i
```

Lists Deployments, Services and Ingresses in the namespace.

### Relationships

```graphql
# Match 2 or more related resources
MATCH (d:Deployment {name: "nginx"})->(rs:ReplicaSet)->(p:Pod)->(s:Service) RETURN s.metadata.name
```

Return the names of Services that expose Pods that are owned by ReplicaSets that are owned by Deployments called "nginx".

### Node with Multiple Properties

```graphql
MATCH (s:Service {type: "LoadBalancer", region: "us-west-1"}) RETURN s.metadata.name, s.status.LoadBalancer
```

Finds Services of type "LoadBalancer" in the "us-west" region and returns their status.

### Updating Resources

```graphql
# RETURN is optional
MATCH (d:Deployment {name: "nginx"}) SET d.metadata.labels.app="nginx-updated" RETURN d
```

### Deleting Resources

```graphql
MATCH (d:Deployment {name: "nginx"}) DELETE d
```

Deletes the "nginx" Deployment.

### Creating Resources

```graphql
# Notice the payload inside CREATE caluse nodes must be valid JSON (quotes surround key names)
# This will be improved in a future version

# Single resource
CREATE (d:Deployment {"name": "nginx", "replicas": 3, "image": "nginx:latest"})

# Create by relationship
MATCH (d:Deployment {name: "nginx"}) CREATE (d)->(s:Service)
```

## Development

Cyphernetes is written in Go and utilizes a parser generated by goyacc to interpret the custom query language.

### Prerequisites

* Go (1.16 or later)
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

* Thanks to the [Neo4j](https://neo4j.com/) community for the inspiration behind the query language.

## Authors

* _Initial work_ - [Avital Tamir](https://github.com/avitaltamir)
