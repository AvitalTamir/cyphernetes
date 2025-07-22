export interface Notebook {
  id: string
  name: string
  owner_id: string
  created_at: string
  updated_at: string
  shared_with: string[]
  is_public: boolean
  tags: string[]
  cells?: Cell[]
}

export interface Cell {
  id: string
  notebook_id: string
  type: CellType
  name?: string
  query: string
  visualization_type: VisualizationType
  refresh_interval: number
  position: number
  row_index: number
  col_index: number
  layout_mode: LayoutMode
  results?: any
  last_executed?: string
  is_running: boolean
  error?: string
  config: CellConfig
}

export type CellType = 'query' | 'markdown' | 'webpage'

// Main visualization modes
export type VisualizationMode = 'document' | 'table' | 'graph' | 'logs'

// Sub-modes for each main mode
export type DocumentMode = 'json' | 'yaml'
export type TableMode = 'default' // Can expand later with pagination, sorting options
export type GraphMode = 'force' | 'pie' | 'bar' // Different graph types
export type LogsMode = 'stream' // Can expand later with different log views

// Legacy type for backward compatibility (will map to modes)
export type VisualizationType = 'json' | 'yaml' | 'table' | 'graph'

export type LayoutMode = 1 | 2 | 4 // Single, Double, Quad

export interface CellConfig {
  // Visualization modes
  visualization_mode?: VisualizationMode
  document_mode?: DocumentMode
  table_mode?: TableMode
  graph_mode?: GraphMode
  logs_mode?: LogsMode
  
  // For table visualization
  page_size?: number
  
  // For graph visualization
  graph_layout?: 'force' | 'circular' | 'tree'
  node_size?: number
  
  // Common settings
  height?: number
  
  // Context settings
  context?: string
  namespace?: string
}

export interface User {
  id: string
  username: string
  display_name: string
  created_at: string
}

export interface Session {
  id: string
  notebook_id: string
  user_id: string
  username: string
  connected_at: string
  last_activity: string
  is_owner: boolean
}

export interface SharePin {
  pin: string
  notebook_id: string
  created_by: string
  created_at: string
  expires_at: string
  wg_endpoint: string
  wg_pubkey: string
}

export interface WebSocketMessage {
  type: string
  [key: string]: any
}

export interface NotebookState {
  notebooks: Notebook[]
  currentNotebook: Notebook | null
  activeSessions: Session[]
  isLoading: boolean
  error: string | null
}