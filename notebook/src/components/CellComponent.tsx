import React, { useState, useCallback } from 'react'
import { Cell, VisualizationType, VisualizationMode, DocumentMode, GraphMode } from '../types/notebook'
import ForceGraph2D from 'react-force-graph-2d'
import * as jsYaml from 'js-yaml'
import { FileText, Table, Network, Edit3, Play, Save, X, Trash2, Search } from 'lucide-react'
import { MarkdownCell } from './MarkdownCell'
import { SyntaxHighlighter } from './SyntaxHighlighter'
import { Prism as PrismSyntaxHighlighter } from 'react-syntax-highlighter'
import { oneLight } from 'react-syntax-highlighter/dist/cjs/styles/prism'
import './CellComponent.css'

// Dynamic node color mapping (same as web client)
const getNodeColor = (nodeType: string): string => {
  const colorMap: {[key: string]: string} = {
    'Pod': '#4285F4',         // Google Blue
    'Service': '#34A853',     // Google Green
    'Deployment': '#FBBC05',  // Google Yellow
    'StatefulSet': '#EA4335', // Google Red
    'ConfigMap': '#8E44AD',   // Purple
    'Secret': '#F39C12',      // Orange
    'PersistentVolumeClaim': '#1ABC9C', // Turquoise
    'Ingress': '#E74C3C',     // Bright Red
    'Job': '#3498DB',         // Light Blue
    'CronJob': '#2ECC71',     // Emerald
    'Namespace': '#9B59B6',   // Amethyst
    'ReplicaSet': '#E67E22',  // Carrot
    'DaemonSet': '#16A085',   // Green Sea
    'Endpoint': '#2980B9',    // Belize Hole
    'Node': '#F1C40F',        // Sunflower
  }
  return colorMap[nodeType] || '#aaaaaa'
}


// Helper function to render graph view
const renderGraphView = (data: any): React.ReactNode => {
  if (!data || typeof data !== 'object') {
    return <div className="empty-graph">No graph data available</div>
  }

  // Check if data already has graph structure
  if (data.graph && data.graph.nodes && data.graph.links) {
    return (
      <ForceGraph2D
        graphData={data.graph}
        nodeLabel="name"
        nodeColor={(node: any) => node.type === 'pod' ? '#ff6b6b' : 
                   node.type === 'service' ? '#4ecdc4' : 
                   node.type === 'deployment' ? '#45b7d1' : '#96ceb4'}
        linkColor={() => '#999'}
        linkWidth={2}
        nodeRelSize={8}
        enableZoomInteraction={true}
        enableNodeDrag={true}
        width={800}
        height={400}
      />
    )
  }

  // Use the graph data if available
  const graphData = data.graph || data.Graph
  if (graphData && graphData.Nodes && graphData.Edges) {
    
    // Use the actual graph structure - construct IDs from Kind/Name to match links
    const nodesMap = new Map()
    graphData.Nodes.forEach((node: any) => {
      const kind = node.Kind || node.kind || node.Type || node.type || 'unknown'
      const name = node.Name || node.name || 'unnamed'
      const nodeId = `${kind}/${name}` // This matches the link format
      
      // Deduplicate nodes by ID
      if (!nodesMap.has(nodeId)) {
        nodesMap.set(nodeId, {
          id: nodeId,
          name: name,
          type: kind,
          namespace: node.Namespace || node.namespace || 'default',
          ...node
        })
      }
    })
    
    const nodes = Array.from(nodesMap.values())
    
    const links = graphData.Edges.map((edge: any) => ({
      source: edge.From || edge.source,
      target: edge.To || edge.target,
      type: edge.Type || edge.type || 'relationship'
    }))
    
    return (
      <ForceGraph2D
        graphData={{ nodes, links }}
        nodeLabel="name"
        nodeColor={(node: any) => getNodeColor(node.type)}
        linkColor={() => '#999'}
        linkWidth={2}
        nodeRelSize={8}
        enableZoomInteraction={true}
        enableNodeDrag={true}
        width={800}
        height={400}
      />
    )
  }
  
  // Fallback: Try to extract nodes and relationships from Cyphernetes result data
  const dataToProcess = data.data || data
  if (dataToProcess && typeof dataToProcess === 'object') {
    const nodes: any[] = []
    const links: any[] = []
    const nodeMap = new Map()
    
    // Extract nodes from all arrays in the data
    Object.values(dataToProcess).forEach((items: any) => {
      if (Array.isArray(items)) {
        items.forEach((item: any, index: number) => {
          if (typeof item === 'object' && item !== null) {
            const nodeId = item.name || item.id || `node-${index}`
            const node = {
              id: nodeId,
              name: item.name || nodeId,
              type: item.kind || item.type || 'unknown',
              namespace: item.namespace || 'default',
              ...item
            }
            
            if (!nodeMap.has(nodeId)) {
              nodeMap.set(nodeId, node)
              nodes.push(node)
            }
          }
        })
      }
    })

    // Create links based on relationships (owner references, labels, etc.)
    nodes.forEach((node: any) => {
      if (node.ownerReferences) {
        node.ownerReferences.forEach((ref: any) => {
          const ownerId = ref.name
          if (nodeMap.has(ownerId)) {
            links.push({
              source: ownerId,
              target: node.id,
              relationship: 'owns'
            })
          }
        })
      }
    })

    if (nodes.length > 0) {
      return (
        <ForceGraph2D
          graphData={{ nodes, links }}
          nodeLabel="name"
          nodeColor={(node: any) => getNodeColor(node.type)}
          linkColor={() => '#999'}
          linkWidth={2}
          nodeRelSize={8}
          enableZoomInteraction={true}
          enableNodeDrag={true}
          width={800}
          height={400}
        />
      )
    }
  }

  // Fallback to JSON view if no graph structure can be extracted
  return (
    <div className="empty-graph">
      <p>No graph structure detected in the data.</p>
      <details>
        <summary>View raw data</summary>
        <pre>{JSON.stringify(data, null, 2)}</pre>
      </details>
    </div>
  )
}

// Helper function to render table view using graph data for grouping
const renderTableView = (data: any, graph?: any, query?: string): React.ReactNode => {
  if (!data || typeof data !== 'object') {
    return <pre>{JSON.stringify(data, null, 2)}</pre>
  }
  
  // Parse the query to extract requested fields from the return clause
  let requestedFields: string[] = []
  if (query) {
    // Match the return clause - handle multi-line queries
    const returnMatch = query.match(/return\s+(.+?)(?:\s+order\s+by|\s+limit|\s+skip|$)/is)
    if (returnMatch) {
      // Extract field list and split by comma
      requestedFields = returnMatch[1]
        .split(',')
        .map(field => field.trim())
        .filter(field => field.length > 0)
    }
  }
  
  // console.log('Query:', query)
  // console.log('Requested fields:', requestedFields)
  
  // Build index of resources by variable
  const resourcesByVariable = new Map<string, any[]>()
  Object.entries(data).forEach(([variable, resources]) => {
    if (Array.isArray(resources)) {
      resourcesByVariable.set(variable, resources)
    }
  })
  
  // Determine columns based on requested fields or all fields
  let columns: string[] = []
  
  if (requestedFields.length > 0) {
    // Extract unique variables from requested fields
    const variablesInQuery = new Set<string>()
    requestedFields.forEach(field => {
      const variable = field.split('.')[0]
      if (resourcesByVariable.has(variable)) {
        variablesInQuery.add(variable)
      }
    })
    
    // Sort variables to ensure consistent ordering
    const sortedVariables = Array.from(variablesInQuery).sort()
    
    // Build columns in order: d.name, d.fields..., s.name, s.fields...
    sortedVariables.forEach(variable => {
      // Add .name first
      columns.push(`${variable}.name`)
      
      // Add other fields for this variable
      requestedFields.forEach(field => {
        if (field.startsWith(`${variable}.`) && !field.endsWith('.name')) {
          columns.push(field)
        }
      })
    })
  } else {
    const allColumns = new Set<string>()
    resourcesByVariable.forEach((resources, variable) => {
      resources.forEach(resource => {
        if (typeof resource === 'object' && resource !== null) {
          const addColumns = (obj: any, prefix: string) => {
            Object.entries(obj).forEach(([key, value]) => {
              const columnName = `${prefix}.${key}`
              allColumns.add(columnName)
              if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
                addColumns(value, columnName)
              }
            })
          }
          addColumns(resource, variable)
        }
      })
    })
    columns.push(...Array.from(allColumns).sort())
  }
  
  // Helper to extract field value from resource
  const extractFieldValue = (resource: any, fieldPath: string) => {
    const parts = fieldPath.split('.')
    let current = resource
    
    // Skip the variable part if it's at the beginning
    const startIndex = resourcesByVariable.has(parts[0]) ? 1 : 0
    
    for (let i = startIndex; i < parts.length; i++) {
      if (!current || typeof current !== 'object') return null
      current = current[parts[i]]
    }
    
    return current
  }
  
  // Build rows
  let rows: Array<Record<string, any>> = []
  
  // Handle both lowercase and uppercase field names
  const edges = graph?.links || graph?.Edges
  const nodes = graph?.nodes || graph?.Nodes
  
  if (graph && edges && Array.isArray(edges) && edges.length > 0) {
    // Build a mapping from Kubernetes resource identifiers to our data
    const k8sResourceMap = new Map<string, {variable: string, resource: any}>()
    
    // Map each resource to its Kubernetes identifier
    resourcesByVariable.forEach((resources, variable) => {
      resources.forEach((resource) => {
        // Build Kubernetes-style identifier (e.g., "Deployment/billing-service")
        const kind = resource.kind || (variable === 'd' ? 'Deployment' : variable === 's' ? 'Service' : 'Unknown')
        const name = resource.name || resource.metadata?.name || ''
        const k8sId = `${kind}/${name}`
        
        k8sResourceMap.set(k8sId, {variable, resource})
      })
    })
    
    // Group resources by their relationships
    const patterns: Array<Map<string, any>> = []
    const visited = new Set<string>()
    
    // Find connected components using edges
    const findConnectedResources = (startId: string): Map<string, any> => {
      const pattern = new Map<string, any>()
      const queue = [startId]
      
      while (queue.length > 0) {
        const currentId = queue.shift()!
        if (visited.has(currentId)) continue
        visited.add(currentId)
        
        const resourceInfo = k8sResourceMap.get(currentId)
        if (resourceInfo) {
          pattern.set(resourceInfo.variable, resourceInfo.resource)
        }
        
        // Find all connected resources
        edges.forEach((edge: any) => {
          const from = edge.From || edge.source
          const to = edge.To || edge.target
          
          if (from === currentId && !visited.has(to)) {
            queue.push(to)
          }
          if (to === currentId && !visited.has(from)) {
            queue.push(from)
          }
        })
      }
      
      return pattern
    }
    
    // Find all connected components
    k8sResourceMap.forEach((_, k8sId) => {
      if (!visited.has(k8sId)) {
        const pattern = findConnectedResources(k8sId)
        if (pattern.size > 0) {
          patterns.push(pattern)
        }
      }
    })
    
    
    // Build rows from patterns
    patterns.forEach((resourceMap) => {
      const row: Record<string, any> = {}
      
      columns.forEach(field => {
        const [variable, ...pathParts] = field.split('.')
        const resource = resourceMap.get(variable)
        
        if (resource) {
          const value = extractFieldValue(resource, field)
          row[field] = value
        }
      })
      
      rows.push(row)
    })
  } else {
    // Fallback: create cartesian product of all resources
    
    const variables = Array.from(resourcesByVariable.keys())
    if (variables.length === 0) {
      return <div className="empty-table">No data to display</div>
    }
    
    // For single variable, just create one row per resource
    if (variables.length === 1) {
      const [variable] = variables
      const resources = resourcesByVariable.get(variable)!
      
      resources.forEach(resource => {
        const row: Record<string, any> = {}
        columns.forEach(field => {
          row[field] = extractFieldValue(resource, field)
        })
        rows.push(row)
      })
    } else {
      // For multiple variables, we need graph data to know relationships
      // For now, show a message
      return (
        <div className="empty-table">
          <p>Multiple variables detected but no relationship data available.</p>
          <p>Cannot determine how to join the results.</p>
        </div>
      )
    }
  }
  
  return (
    <table className="results-table">
      <thead>
        <tr>
          {columns.map(col => (
            <th key={col}>{col}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, index) => (
          <tr key={index}>
            {columns.map(col => (
              <td key={col}>
                {row[col] !== null && row[col] !== undefined 
                  ? (typeof row[col] === 'object' 
                    ? JSON.stringify(row[col]) 
                    : String(row[col]))
                  : ''
                }
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}

interface CellComponentProps {
  cell: Cell
  onUpdate: (cellId: string, updates: Partial<Cell>) => void
  onDelete: (cellId: string) => void
  onDragStart?: (cellId: string) => void
  onDragEnd?: () => void
  onDragOver?: (cellId: string) => void
  onDrop?: (cellId: string) => void
  isDragging?: boolean
  isDragOver?: boolean
}

export const CellComponent: React.FC<CellComponentProps> = ({
  cell,
  onUpdate,
  onDelete,
  onDragStart,
  onDragEnd,
  onDragOver,
  onDrop,
  isDragging,
  isDragOver,
}) => {
  // If this is a markdown cell, use the specialized component
  if (cell.type === 'markdown') {
    return (
      <MarkdownCell
        cell={cell}
        onUpdate={onUpdate}
        onDelete={onDelete}
        onDragStart={onDragStart}
        onDragEnd={onDragEnd}
        onDragOver={onDragOver}
        onDrop={onDrop}
        isDragging={isDragging}
        isDragOver={isDragOver}
      />
    )
  }

  // Otherwise, render the query cell
  const [isEditing, setIsEditing] = useState(false)
  const [query, setQuery] = useState(cell.query)

  const handleSave = async () => {
    try {
      const response = await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ query }),
      })
      
      if (response.ok) {
        onUpdate(cell.id, { query })
        setIsEditing(false)
      }
    } catch (error) {
      console.error('Failed to save cell:', error)
    }
  }

  const handleCancel = () => {
    setQuery(cell.query)
    setIsEditing(false)
  }

  const handleExecute = async () => {
    try {
      onUpdate(cell.id, { is_running: true })
      
      const response = await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}/execute`, {
        method: 'POST',
      })
      
      if (response.ok) {
        const result = await response.json()
        onUpdate(cell.id, { 
          results: result, 
          is_running: false,
          last_executed: new Date().toISOString(),
          error: undefined
        })
      } else {
        onUpdate(cell.id, { 
          is_running: false,
          error: 'Failed to execute query'
        })
      }
    } catch (error) {
      onUpdate(cell.id, { 
        is_running: false,
        error: 'Network error'
      })
    }
  }

  // Convert legacy visualization_type to new mode system
  const getVisualizationMode = (): VisualizationMode => {
    const vizType = cell.visualization_type
    if (vizType === 'json' || vizType === 'yaml') return 'document'
    if (vizType === 'table') return 'table'
    if (vizType === 'graph') return 'graph'
    return 'document'
  }

  const getDocumentMode = (): DocumentMode => {
    return cell.config?.document_mode || (cell.visualization_type === 'yaml' ? 'yaml' : 'json')
  }

  const getGraphMode = (): GraphMode => {
    return cell.config?.graph_mode || 'force'
  }

  const [currentMode, setCurrentMode] = useState<VisualizationMode>(getVisualizationMode())
  const [documentMode, setDocumentMode] = useState<DocumentMode>(getDocumentMode())
  const [graphMode, setGraphMode] = useState<GraphMode>(getGraphMode())

  const handleModeChange = async (mode: VisualizationMode) => {
    setCurrentMode(mode)
    
    // Map back to legacy visualization_type for backend compatibility
    let vizType: VisualizationType
    if (mode === 'document') {
      vizType = documentMode
    } else if (mode === 'table') {
      vizType = 'table'
    } else {
      vizType = 'graph'
    }
    
    try {
      const response = await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ 
          visualization_type: vizType,
          config: {
            ...cell.config,
            visualization_mode: mode,
            document_mode: documentMode,
            graph_mode: graphMode
          }
        }),
      })
      
      if (response.ok) {
        onUpdate(cell.id, { 
          visualization_type: vizType,
          config: {
            ...cell.config,
            visualization_mode: mode,
            document_mode: documentMode,
            graph_mode: graphMode
          }
        })
      }
    } catch (error) {
      console.error('Failed to update visualization mode:', error)
    }
  }

  const handleDocumentModeChange = async (mode: DocumentMode) => {
    setDocumentMode(mode)
    
    try {
      const response = await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ 
          visualization_type: mode,
          config: {
            ...cell.config,
            document_mode: mode
          }
        }),
      })
      
      if (response.ok) {
        onUpdate(cell.id, { 
          visualization_type: mode,
          config: {
            ...cell.config,
            document_mode: mode
          }
        })
      }
    } catch (error) {
      console.error('Failed to update document mode:', error)
    }
  }

  const handleGraphModeChange = async (mode: GraphMode) => {
    setGraphMode(mode)
    
    try {
      const response = await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ 
          config: {
            ...cell.config,
            graph_mode: mode
          }
        }),
      })
      
      if (response.ok) {
        onUpdate(cell.id, { 
          config: {
            ...cell.config,
            graph_mode: mode
          }
        })
      }
    } catch (error) {
      console.error('Failed to update graph mode:', error)
    }
  }

  const handleDragStart = (e: React.DragEvent) => {
    e.dataTransfer.setData('text/plain', cell.id)
    e.dataTransfer.effectAllowed = 'move'
    
    // Find the cell container for the drag image
    const cellElement = (e.target as HTMLElement).closest('.cell')
    if (cellElement) {
      e.dataTransfer.setDragImage(cellElement as HTMLElement, 20, 20)
    }
    
    onDragStart?.(cell.id)
  }

  const handleDragEnd = (e: React.DragEvent) => {
    onDragEnd?.()
  }

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    onDragOver?.(cell.id)
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    onDrop?.(cell.id)
  }

  // Determine execution status
  const getExecutionStatus = () => {
    if (!cell.last_executed) return ''
    if (cell.error) return 'executed-error'
    return 'executed-success'
  }

  return (
    <div 
      className={`cell ${isEditing ? 'editing' : getExecutionStatus()} ${isDragging ? 'dragging' : ''} ${isDragOver ? 'drag-over' : ''}`}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
    >
      <div 
        className="cell-header"
        draggable={!isEditing}
        onDragStart={handleDragStart}
        onDragEnd={handleDragEnd}
        title="Drag to reorder"
        style={{ cursor: !isEditing ? 'grab' : 'default' }}
      >
        <div className="cell-info">
          <span className="cell-type">
            <Search size={12} />
            Query
          </span>
          {cell.last_executed && (
            <span className="cell-executed">
              Last executed: {new Date(cell.last_executed).toLocaleTimeString()}
            </span>
          )}
        </div>
        <div className="cell-actions">
          <div className="visualization-modes">
            <button
              className={`mode-icon ${currentMode === 'document' ? 'active' : ''}`}
              onClick={() => handleModeChange('document')}
              title="Document View"
            >
              <FileText size={16} />
            </button>
            <button
              className={`mode-icon ${currentMode === 'table' ? 'active' : ''}`}
              onClick={() => handleModeChange('table')}
              title="Table View"
            >
              <Table size={16} />
            </button>
            <button
              className={`mode-icon ${currentMode === 'graph' ? 'active' : ''}`}
              onClick={() => handleModeChange('graph')}
              title="Graph View"
            >
              <Network size={16} />
            </button>
          </div>
          
          {isEditing ? (
            <>
              <button onClick={handleSave} className="cell-action save">
                <Save size={14} />
                Save
              </button>
              <button onClick={handleCancel} className="cell-action cancel">
                <X size={14} />
                Cancel
              </button>
            </>
          ) : (
            <>
              <button onClick={() => setIsEditing(true)} className="cell-action edit">
                <Edit3 size={14} />
                Edit
              </button>
              <button 
                onClick={handleExecute} 
                disabled={cell.is_running}
                className="cell-action execute"
              >
                <Play size={14} />
                {cell.is_running ? 'Running...' : 'Run'}
              </button>
              <button onClick={() => onDelete(cell.id)} className="cell-action delete">
                <Trash2 size={14} />
                Delete
              </button>
            </>
          )}
        </div>
      </div>

      <div className="cell-content">
        {isEditing ? (
          <div className="cell-editor-container">
            <div className="cell-editor-wrapper">
              <textarea
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Enter your Cyphernetes query here..."
                className="cell-editor cell-editor-overlay"
                rows={6}
              />
              <div className="cell-editor-highlight">
                <PrismSyntaxHighlighter
                  language="cypher"
                  style={oneLight}
                  customStyle={{
                    margin: 0,
                    padding: '12px',
                    fontSize: '14px',
                    lineHeight: '1.4',
                    backgroundColor: 'transparent',
                    border: 'none',
                    borderRadius: '4px'
                  }}
                >
                  {query || ' '}
                </PrismSyntaxHighlighter>
              </div>
            </div>
          </div>
        ) : (
          <div className="cell-query-display">
            {cell.query ? (
              <PrismSyntaxHighlighter
                language="cypher"
                style={oneLight}
                customStyle={{
                  margin: 0,
                  padding: '12px',
                  fontSize: '14px',
                  lineHeight: '1.4',
                  backgroundColor: '#f8f9fa',
                  border: 'none',
                  borderRadius: '4px',
                  color: '#4c4f69'
                }}
              >
                {cell.query}
              </PrismSyntaxHighlighter>
            ) : (
              <span className="cell-placeholder">No query yet</span>
            )}
          </div>
        )}
      </div>

      {cell.error && (
        <div className="cell-error">
          <strong>Error:</strong> {cell.error}
        </div>
      )}

      {cell.results && (
        <div className="cell-results">
          <div className="cell-results-header">
            <span>Results</span>
            <div className="mode-options">
              {currentMode === 'document' && (
                <div className="document-options">
                  <button
                    className={`option-btn ${documentMode === 'json' ? 'active' : ''}`}
                    onClick={() => handleDocumentModeChange('json')}
                  >
                    JSON
                  </button>
                  <button
                    className={`option-btn ${documentMode === 'yaml' ? 'active' : ''}`}
                    onClick={() => handleDocumentModeChange('yaml')}
                  >
                    YAML
                  </button>
                </div>
              )}
              {currentMode === 'table' && (
                <div className="table-options">
                  <span className="option-label">Default view</span>
                </div>
              )}
              {currentMode === 'graph' && (
                <div className="graph-options">
                  <button
                    className={`option-btn ${graphMode === 'force' ? 'active' : ''}`}
                    onClick={() => handleGraphModeChange('force')}
                  >
                    Force
                  </button>
                  <button
                    className={`option-btn ${graphMode === 'pie' ? 'active' : ''}`}
                    onClick={() => handleGraphModeChange('pie')}
                  >
                    Pie
                  </button>
                  <button
                    className={`option-btn ${graphMode === 'tree' ? 'active' : ''}`}
                    onClick={() => handleGraphModeChange('tree')}
                  >
                    Tree
                  </button>
                </div>
              )}
            </div>
          </div>
          <div className="cell-results-content">
            {currentMode === 'document' && (
              <>
                {documentMode === 'json' && (
                  <SyntaxHighlighter
                    code={JSON.stringify(cell.results?.data || cell.results, null, 2)}
                    language="json"
                  />
                )}
                {documentMode === 'yaml' && (
                  <SyntaxHighlighter
                    code={jsYaml.dump(cell.results?.data || cell.results)}
                    language="yaml"
                  />
                )}
              </>
            )}
            {currentMode === 'table' && (
              <div className="table-output">
                {renderTableView(cell.results?.data || cell.results, cell.results?.graph, cell.query)}
              </div>
            )}
            {currentMode === 'graph' && (
              <div className="graph-output">
                {graphMode === 'force' && renderGraphView(cell.results)}
                {graphMode === 'pie' && (
                  <div className="pie-chart-placeholder">
                    <p>Pie chart visualization coming soon...</p>
                    <div className="fallback-view">
                      {renderGraphView(cell.results)}
                    </div>
                  </div>
                )}
                {graphMode === 'tree' && (
                  <div className="tree-chart-placeholder">
                    <p>Tree visualization coming soon...</p>
                    <div className="fallback-view">
                      {renderGraphView(cell.results)}
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}