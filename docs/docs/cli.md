---
sidebar_position: 1
---

# CLI

> Note: Dry Run mode is available for all CLI commands.

  The `-d, --dry-run` flag can be used with any CLI command to enable dry run mode.
  When dry run mode is enabled, Cyphernetes will print the actions it would take without actually performing them.

  ```bash
  cyphernetes --dry-run query 'CREATE (d:Deployment {name: "nginx"})'
  cyphernetes --dry-run shell
  cyphernetes --dry-run web
  ```

## Shell

Cyphernetes comes with a shell that lets you interactively query the Kubernetes API using Cyphernetes.

To start the shell, run:

```bash
cyphernetes shell
```

The shell supports syntax highlighting, autocompletion, and history.
Use tab to autocomplete keywords, labels, and jsonPaths.

By default the shell works in multiline mode, which means your query will be executed when you type a semicolon (`;`).
You can toggle multiline mode by typing `\m` in the shell.

At any time, you can type `exit` to exit the shell, or `help` to get a list of available commands.

Available shell commands:

* `help` - Display help and documentation.
* `exit` - Exit the shell.
* `\n <namespace>|all` - Set the namespace context for the shell to either `<namespace>` or all namespaces.
* `\m` - Toggle multiline mode (execute query on ';').
* `\g` - Toggle graph mode (print graph as ASCII art).
* `\gl` - Toggle graph layout (Left to Right or Top to Bottom).
* `\d` - Print debug information.
* `\q` - Toggle printing query execution time.
* `\r` - Toggle raw output (disable colorized JSON).
* `\cc` - Clear the cache.
* `\pc` - Print the cache.
* `\lm` - List available macros.
* `:macro_name [args]` - Execute a macro.

### Graphs

Cyphernetes can print the Kubernetes resource graph as an ASCII graph.
To toggle printing the graph, use the `\g` command.
To change the graph layout, use the `\gl` command.

### Macros

Cyphernetes comes with a set of default macros that can be used to query the Kubernetes API.

There are many built-in macros for performing common tasks such as listing pods, services, deployments, etc. as well as for performing common tasks such as exposing a deployment as a service.

You can list available macros by running `\lm` in the shell.

You can use a macro by running `:<macro-name>` in the shell:

```graphql
> :getpo

{
  "pods": [
    {
      "Age": "2024-08-06T21:29:05Z",
      "IP": "10.244.0.5",
      "Name": "nginx-bf5d5cf98-m69mz",
      "Node": "kind-control-plane",
      "Status": "Running"
    }
  ]
}

Macro executed in 14.971875ms
```

User macros are defined in the `~/.cyphernetes/macros` file.
Macros are defined using the following syntax:

```
:macro <name> [<args>] [// description]
MATCH (p:Pods) RETURN p.metadata.name;

// Multi-line queries are supported
:macro my-macro // Return all pod names
MATCH (p:Pods)
RETURN p.metadata.name;
```

----

## Query

The `query` command lets you run a single Cyphernetes query from the command line.
Available flags:

* `-r, --raw-output` - Disable colorized JSON output.

```bash
cyphernetes query 'MATCH (d:Deployment {name: "nginx"}) RETURN d'
```

## Web

The `web` command starts a web server that lets you interact with Cyphernetes using a web interface.

To start the web server, run:

```bash
cyphernetes web
```

You can then visit `http://localhost:8080` in your browser to interact with Cyphernetes.

## Custom Relationships

Cyphernetes allows defining custom relationships between Kubernetes resources in a `~/.cyphernetes/relationships.yaml` file. This is useful when working with custom resources or when you want to define relationships that aren't built into Cyphernetes.

Example relationships.yaml:

```yaml
relationships:
  - kindA: applications.argoproj.io
    kindB: services
    relationship: ARGOAPP_SYNC_SERVICE
    matchCriteria:
      - fieldA: "$.spec.source.targetRevision"
        fieldB: "$.metadata.labels.targetRevision"
        comparisonType: ExactMatch
      - fieldA: "$.spec.project"
        fieldB: "$.metadata.labels.project" 
        comparisonType: ExactMatch

  - kindA: pods
    kindB: deployments
    relationship: DEPLOYMENT_OWN_POD
    matchCriteria:
      - fieldA: "$.metadata.name"
        fieldB: "$.metadata.name"
        comparisonType: StringContains
```

The relationships.yaml file supports the following fields:

- `kindA`, `kindB`: The Kubernetes resource kinds to relate (use plural form, e.g. "deployments" not "Deployment")
- `relationship`: A unique identifier for this relationship type (conventionally UPPERCASE)
- `matchCriteria`: List of criteria that must all match for the relationship to exist
  - `fieldA`: JSONPath to field in kindA resource
  - `fieldB`: JSONPath to field in kindB resource  
  - `comparisonType`: One of:
    - `ExactMatch`: Values must match exactly
    - `ContainsAll`: All key-value pairs in fieldB must exist in fieldA
    - `StringContains`: The value in fieldA contains the value in fieldB as a substring
  - `defaultProps`: Optional default values to use when creating resources
    - `fieldA`: JSONPath to field in kindA
    - `fieldB`: JSONPath to field in kindB  
    - `default`: Default value if field is not specified

Custom relationships are loaded on startup and can be used just like built-in relationships in queries:

```graphql
MATCH (d:Deployment)->(p:Pod)
RETURN d.metadata.name, p.metadata.name
```
