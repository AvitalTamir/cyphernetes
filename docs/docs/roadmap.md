---
sidebar_position: 8
---

# Roadmap

## Initial Project Setup ✅

- ✅ Initialize the project repository
- ✅ Set up version control with Git
- ✅ Create and document the project directory structure
- ✅ Choose a Go package management tool and initialize the package
- ✅ Set up a Go workspace with the necessary Go modules

## Tooling and Framework ✅

- ✅ Set up a testing framework using Go's built-in testing package
- ✅ Configure a continuous integration service
- ✅ Establish linting and code formatting tools
- ✅ Implement logging and debug output mechanisms

## Lexer and Parser Development ✅

- ✅ Create the basic lexer with support for initial tokens
- ✅ Develop a yacc file for the initial grammar rules
- ✅ Write unit tests for basic tokenization
- ✅ Implement a basic parser to handle `MATCH` queries
- ✅ Test and debug the lexer and parser with simple queries

## Kubernetes Client Integration 🚧

- ✅ Evaluate and select a Go Kubernetes client library
- ✅ Set up authentication and configuration for accessing a Kubernetes cluster
- ✅ Implement a wrapper around the Kubernetes client to execute basic queries
- ✅ Develop mapping logic to convert parsed queries into Kubernetes API calls
- ✅ Add support for complex queries involving multiple Kubernetes resources
- 🚧 Test Kubernetes client integration with mock and real clusters

## Expanding Lexer and Parser 🚧

- ✅ Add support for additional tokens (e.g., braces, commas, relationship types)
- ✅ Extend grammar rules to cover node properties and relationships
- ✅ Implement parsing logic for `CREATE`, `SET`, and `DELETE` keywords
- 🚧 Refine error handling for syntax and parsing errors
- 🚧 Optimize lexer and parser for performance

## Interactive Shell Interface 🚧

- ✅ Basic shell interface for inputting queries and displaying results
- ✅ Syntax highlighting
- ✅ Autocompletion
- ✅ Add help and documentation to the shell
- 🚧 Test shell with various input scenarios

## Cypher-Like Query Language Parser Roadmap

### Phase 1: Basic MATCH Support ✅
- ✅ Support for basic `MATCH` queries (e.g., `MATCH (k:Kind)`)
- ✅ Write unit tests for basic `MATCH` query parsing

### Phase 2: RETURN Clause ✅
- ✅ Implement parsing of the `RETURN` clause
- ✅ Update the lexer to recognize the `RETURN` keyword
- ✅ Extend the yacc grammar to include `RETURN` statement rules
- ✅ Write unit tests for queries with `RETURN` clauses

### Phase 3: Node Properties ✅
- ✅ Extend the parser to handle node properties
- ✅ Update the lexer to recognize curly braces and commas
- ✅ Update the yacc file to handle node properties syntax
- ✅ Write unit tests for `MATCH` queries with node properties

### Phase 4: Relationships ✅
- ✅ Support parsing of relationships in `MATCH` queries
- ✅ Extend the yacc grammar to handle relationship patterns
- ✅ Write unit tests for `MATCH` queries involving relationships
- ✅ Support relationships between more than 2 nodes
- ✅ Update the lexer to recognize relationship pattern tokens (e.g., `-[]->`)

### Phase 5: Advanced MATCH Support ✅
- ✅ Match Clauses to contain NodePatternLists instead of a single tuple of Node/ConnectedNode
- ✅ Support more than 2 comma-separated NodePatternLists

### Phase 6: SET Clause ✅
- ✅ Implement parsing of the `SET` clause
- ✅ Update the lexer to recognize the `SET` keyword and property assignment syntax
- ✅ Extend the yacc grammar to include `SET` statement rules
- ✅ Write unit tests for queries with `SET` clauses

### Phase 7: DELETE Statement ✅
- ✅ Add support for `DELETE` statements
- ✅ Update the lexer to recognize the `DELETE` keyword
- ✅ Extend the yacc grammar to parse `DELETE` statements
- ✅ Write unit tests for `DELETE` statement parsing

### Phase 8: CREATE Statement ✅
- ✅ Add support for `CREATE` statements
- ✅ Update the lexer to recognize the `CREATE` keyword
- ✅ Extend the yacc grammar to parse `CREATE` statements
- ✅ Write unit tests for `CREATE` statement parsing

### Phase 9: WHERE Clause ✅
- ✅ Add support for `WHERE` clauses
- ✅ Update the lexer to recognize the `WHERE` keyword
- ✅ Extend the yacc grammar to parse `WHERE` clauses
- ✅ Write unit tests for `WHERE` clause parsing

### Phase 10: AS Clause ✅
- ✅ Add support for `AS` clauses
- ✅ Update the lexer to recognize the `AS` keyword
- ✅ Extend the yacc grammar to parse `AS` clauses
- ✅ Write unit tests for `AS` clause parsing