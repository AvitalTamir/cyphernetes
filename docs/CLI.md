# The Cyphernetes CLI

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

### Using a Macro

You can use a macro by running `:<macro-name>` in the shell:

```graphql
> :po

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

### Creating a Macro

User macros are defined in the `~/.cyphernetes/macros` file.
Macros are defined using the following syntax:

```
:macro <name> [<args>]
MATCH (p:Pods) RETURN p.metadata.name;

# Multi-line queries are supported
:macro my-macro
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
