# Project Roadmap

### Initial Project Setup

- [x] Initialize the project repository.
- [x] Set up version control with Git.
- [x] Create and document the project directory structure.
- [x] Choose a Go package management tool and initialize the package.
- [x] Set up a Go workspace with the necessary Go modules.

### Tooling and Framework

- [x] Set up a testing framework using Go's built-in testing package.
- [x] Configure a continuous integration service.
- [x] Establish linting and code formatting tools.
- [x] Implement logging and debug output mechanisms.

### Lexer and Parser Development

- [x] Create the basic lexer with support for initial tokens.
- [x] Develop a yacc file for the initial grammar rules.
- [x] Write unit tests for basic tokenization.
- [x] Implement a basic parser to handle `MATCH` queries.
- [x] Test and debug the lexer and parser with simple queries.

### Kubernetes Client Integration

- [x] Evaluate and select a Go Kubernetes client library.
- [x] Set up authentication and configuration for accessing a Kubernetes cluster.
- [x] Implement a wrapper around the Kubernetes client to execute basic queries.
- [x] Develop mapping logic to convert parsed queries into Kubernetes API calls.
- [x] Add support for complex queries involving multiple Kubernetes resources.
- [ ] Test Kubernetes client integration with mock and real clusters.

### Expanding Lexer and Parser

- [x] Add support for additional tokens (e.g., braces, commas, relationship types).
- [x] Extend grammar rules to cover node properties and relationships.
- [ ] Implement parsing logic for `CREATE`, `SET`, and `DELETE` keywords.
- [ ] Refine error handling for syntax and parsing errors.
- [ ] Optimize lexer and parser for performance.

### Interactive Shell Interface

- [x] Basic shell interface for inputting queries and displaying results.
- [x] Syntax highlighting.
- [x] Autocompletion.
- [ ] Add help and documentation to the shell.
- [ ] Test shell with various input scenarios.

## Cypher-Like Query Language Parser Roadmap

The goal of this roadmap is to incrementally develop a parser that can handle a Cypher-like query language. The final version should support complex queries involving `MATCH`, `RETURN`, `CREATE`, `SET`, and `DELETE` statements.

### Phase 1: Basic MATCH Support

- [x] Support for basic `MATCH` queries (e.g., `MATCH (k:Kind)`).
- [x] Write unit tests for basic `MATCH` query parsing.

### Phase 2: RETURN Clause

- [x] Implement parsing of the `RETURN` clause.
- [x] Update the lexer to recognize the `RETURN` keyword.
- [x] Extend the yacc grammar to include `RETURN` statement rules.
- [x] Write unit tests for queries with `RETURN` clauses.

### Phase 3: Node Properties

- [x] Extend the parser to handle node properties.
- [x] Update the lexer to recognize curly braces and commas.
- [x] Update the yacc file to handle node properties syntax.
- [x] Write unit tests for `MATCH` queries with node properties.

### Phase 4: Relationships

- [x] Support parsing of relationships in `MATCH` queries.
- [x] Extend the yacc grammar to handle relationship patterns.
- [x] Write unit tests for `MATCH` queries involving relationships.
- [x] Support relationships between more than 2 nodes.
- [x] Update the lexer to recognize relationship pattern tokens (e.g., `-[]->`).

### Phase 5: Advanced MATCH Support
- [x] Match Clauses to contain NodePatternLists instead of a single tuple of Node/ConnectedNode
- [x] Support more than 2 comma-separated NodePatternLists.

### Phase 6: SET Clause

- [x] Implement parsing of the `SET` clause.
- [x] Update the lexer to recognize the `SET` keyword and property assignment syntax.
- [x] Extend the yacc grammar to include `SET` statement rules.
- [x] Write unit tests for queries with `SET` clauses.

### Phase 7: DELETE Statement

- [x] Add support for `DELETE` statements.
- [x] Update the lexer to recognize the `DELETE` keyword.
- [x] Extend the yacc grammar to parse `DELETE` statements.
- [x] Write unit tests for `DELETE` statement parsing.

### Phase 8: CREATE Statement

- [ ] Add support for `CREATE` statements.
- [ ] Update the lexer to recognize the `CREATE` keyword.
- [ ] Extend the yacc grammar to parse `CREATE` statements.
- [ ] Write unit tests for `CREATE` statement parsing.

### Phase 9: WHERE Clause

- [ ] Add support for `WHERE` clauses.
- [ ] Update the lexer to recognize the `WHERE` keyword.
- [ ] Extend the yacc grammar to parse `WHERE` clauses.
- [ ] Write unit tests for `WHERE` clause parsing.

### Phase 10: AS Clause

- [ ] Add support for `AS` clauses.
- [ ] Update the lexer to recognize the `AS` keyword.
- [ ] Extend the yacc grammar to parse `AS` clauses.
- [ ] Write unit tests for `AS` clause parsing.

### Phase 11: Complex Query Parsing

- [ ] Combine all elements to support full query parsing.
- [ ] Ensure the lexer and yacc grammar can handle complex queries with multiple clauses.
- [ ] Write unit tests for parsing full queries including `MATCH`, `RETURN`, `CREATE`, `SET`, and `DELETE`.
