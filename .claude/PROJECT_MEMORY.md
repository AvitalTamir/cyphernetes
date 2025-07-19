# Cyphernetes Project Memory

This file serves as Claude's persistent memory for the massive new Cyphernetes program development.

## Project Status
- **Started**: 2025-07-18
- **Current Phase**: Initial Planning
- **Last Updated**: 2025-07-18

## Project Overview
**Project Name**: Cyphernetes Notebooks

**Vision**: Build a Jupyter-style notebook interface for Cyphernetes that enables:
- Multiple queries running continuously in dashboard-style cells
- Multiple visualization formats (JSON, YAML, Table, various graph formats)
- Real-time multi-user collaboration
- Invite system for sharing notebooks
- Persistent notebooks that can be saved and shared

**Key Features**:
1. Notebook-based interface with cells containing Cyphernetes queries
2. Live/continuous query execution with auto-refresh capabilities
3. Multiple visualization options per cell
4. Real-time collaboration (multiple users editing/viewing simultaneously)
5. User authentication and invitation system
6. Dashboard-style layout with customizable cell arrangements

## Key Decisions

### Architecture Decisions (2025-07-18)

1. **Deployment Model**: Decentralized, user-owned notebooks
   - Runs locally via `cyphernetes notebook` command
   - No centralized server - each user runs their own instance
   - Uses local Kubernetes config/permissions where it runs

2. **Networking**: Peer-to-peer via WireGuard
   - Users connect directly to notebook owners via WireGuard
   - Optional cyphernet.es dynamic DNS for peer discovery
   - No traditional authentication - uses temporary pin codes

3. **Storage**: SQLite for notebook persistence
   - Perfect for single-instance deployments
   - Handles concurrent access well for small teams
   - Easy backup/restore

4. **UI Layout**: Jupyter-style with flexible rows
   - Linear notebook flow (not grid dashboard)
   - Rows can be split into 1, 2, or 4 cells
   - Simpler responsive design

5. **Integration**: New command in main CLI
   - `cyphernetes notebook` launches the notebook server
   - Intended to eventually replace current web UI
   - Shares core query engine with existing codebase

## Progress Tracking
*Track completed tasks and milestones*

## Open Questions

1. **Deployment Model**:
   - Should this be a separate service or integrated into the existing web UI?
   - How do we handle scaling for multiple concurrent users?

2. **Authentication & Authorization**:
   - Build our own auth system or integrate with existing solutions (OAuth, OIDC)?
   - How do we handle Kubernetes RBAC passthrough for queries?
   - Should each user run queries with their own K8s credentials?

3. **Storage Backend**:
   - File-based (simple, like Jupyter) or database (better for collaboration)?
   - If database, embedded (SQLite) or external (PostgreSQL)?
   - How do we handle notebook versioning/history?

4. **Real-time Collaboration**:
   - Use WebSockets or Server-Sent Events?
   - CRDT (Conflict-free Replicated Data Types) or Operational Transforms?
   - How do we handle concurrent edits to the same cell?

5. **Query Execution**:
   - Should continuous queries use polling or streaming?
   - How do we handle resource limits (prevent runaway queries)?
   - Query result caching strategy?

6. **UI/UX Considerations**:
   - Grid layout (like Grafana) or vertical notebook style (like Jupyter)?
   - How do we handle responsive design for dashboards?
   - Cell resize/reorder capabilities?

## Technical Notes
*Important technical details discovered during development*

## Dependencies & Blockers
*Track any dependencies or blockers encountered*