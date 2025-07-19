import React, { useState, useCallback, useEffect, memo } from 'react'
import { Cell, VisualizationType, VisualizationMode, DocumentMode, GraphMode } from '../types/notebook'
import ForceGraph2D from 'react-force-graph-2d'
import * as jsYaml from 'js-yaml'
import { FileText, Table, Network, Edit3, Play, Pause, Save, X, Trash2, Search, ChevronDown, ChevronUp } from 'lucide-react'
import { MarkdownCell } from './MarkdownCell'
import { SyntaxHighlighter } from './SyntaxHighlighter'
import { ContextSelector } from './ContextSelector'
import { Prism as PrismSyntaxHighlighter } from 'react-syntax-highlighter'
import { oneLight } from 'react-syntax-highlighter/dist/cjs/styles/prism'
import './CellComponent.css'

// Simple ForceGraph wrapper with debug logging
const DebugForceGraph = memo(({ graphData, cellId }: { 
  graphData: any, 
  cellId: string
}) => {
  
  // Use the same color function as web client, but for notebook data structure
  const getNodeColor = (node: any) => {
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
    // Notebook uses 'type' field, web client uses 'kind' field
    return colorMap[node.type] || '#aaaaaa'
  }
  
  return (
    <ForceGraph2D
      key={`force-${cellId}`}
      graphData={graphData}
      nodeLabel="name"
      nodeColor={getNodeColor}
      linkColor={() => '#999'}
      linkWidth={2}
      nodeRelSize={8}
      enableZoomInteraction={true}
      enableNodeDrag={true}
      width={800}
      height={400}
    />
  )
})

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


// Helper function to render pie chart view
const renderPieChartView = (data: any, query?: string): React.ReactNode => {
  if (!data || typeof data !== 'object') {
    return <div className="empty-pie">No data available for pie chart</div>
  }

  // Parse the query to detect single vs multiple return elements
  let requestedFields: string[] = []
  if (query) {
    const returnMatch = query.match(/return\s+(.+?)(?:\s+order\s+by|\s+limit|\s+skip|$)/is)
    if (returnMatch) {
      requestedFields = returnMatch[1]
        .split(',')
        .map(field => field.trim())
        .filter(field => field.length > 0)
    }
  }

  // Check if more than one element is returned
  if (requestedFields.length > 1) {
    return (
      <div className="pie-chart-error">
        <p>Pie charts are only supported for queries with a single return element.</p>
        <p>Your query returns {requestedFields.length} elements: {requestedFields.join(', ')}</p>
      </div>
    )
  }

  // Extract data for pie chart
  const dataToProcess = data.data || data
  if (!dataToProcess || typeof dataToProcess !== 'object') {
    return <div className="empty-pie">No data structure suitable for pie chart</div>
  }

  // Get the single variable data
  const entries = Object.entries(dataToProcess)
  if (entries.length === 0) {
    return <div className="empty-pie">No data to display in pie chart</div>
  }

  // Use the first (and ideally only) variable's data
  const [variableName, resources] = entries[0]
  if (!Array.isArray(resources)) {
    return <div className="empty-pie">Data is not in array format suitable for pie chart</div>
  }

  // Extract the field path from the return clause to use for grouping
  let groupingField: string | null = null
  if (requestedFields.length === 1) {
    const field = requestedFields[0]
    // Extract the property path after the variable (e.g., "p.status.phase" -> "status.phase")
    const dotIndex = field.indexOf('.')
    if (dotIndex !== -1) {
      groupingField = field.substring(dotIndex + 1)
    }
  }


  // Helper function to extract nested field value
  const extractNestedValue = (obj: any, path: string): string => {
    const parts = path.split('.')
    let current = obj
    
    for (const part of parts) {
      if (!current || typeof current !== 'object') return 'Unknown'
      current = current[part]
    }
    
    // Convert result to string
    if (typeof current === 'string') return current
    if (typeof current === 'number') return current.toString()
    if (typeof current === 'boolean') return current.toString()
    if (current === null || current === undefined) return 'null'
    return 'Unknown'
  }

  // Count occurrences by the requested field or fallback logic
  const counts: Record<string, number> = {}
  resources.forEach((resource: any) => {
    if (typeof resource === 'object' && resource !== null) {
      let groupBy = 'Unknown'
      
      if (groupingField) {
        // Use the specific field requested in the return clause
        groupBy = extractNestedValue(resource, groupingField)
      } else {
        // Fallback to default grouping logic
        if (typeof resource.kind === 'string') {
          groupBy = resource.kind
        } else if (typeof resource.type === 'string') {
          groupBy = resource.type
        } else if (typeof resource.name === 'string') {
          groupBy = resource.name
        }
      }
      
      counts[groupBy] = (counts[groupBy] || 0) + 1
    }
  })

  const chartData = Object.entries(counts).map(([name, value]) => ({ name, value }))
  
  if (chartData.length === 0) {
    return <div className="empty-pie">No categorical data found for pie chart</div>
  }

  // Simple pie chart implementation using SVG
  const total = chartData.reduce((sum, item) => sum + item.value, 0)
  let currentAngle = 0
  const radius = 120
  const centerX = 160
  const centerY = 160

  const colors = ['#4285F4', '#34A853', '#FBBC05', '#EA4335', '#8E44AD', '#F39C12', '#1ABC9C', '#E74C3C']

  return (
    <div className="pie-chart-container">
      <svg width="400" height="320" viewBox="0 0 320 320">
        {chartData.map((item, index) => {
          const percentage = item.value / total
          const angle = percentage * 2 * Math.PI
          
          // Start angle for this slice
          const startAngle = currentAngle
          const endAngle = currentAngle + angle
          
          const x1 = centerX + radius * Math.cos(startAngle)
          const y1 = centerY + radius * Math.sin(startAngle)
          const x2 = centerX + radius * Math.cos(endAngle)
          const y2 = centerY + radius * Math.sin(endAngle)
          
          const largeArcFlag = angle > Math.PI ? 1 : 0
          
          // For single slice covering the whole pie, draw a circle
          let pathData
          if (chartData.length === 1) {
            pathData = [
              `M ${centerX} ${centerY}`,
              `m -${radius}, 0`,
              `a ${radius},${radius} 0 1,1 ${radius * 2},0`,
              `a ${radius},${radius} 0 1,1 -${radius * 2},0`
            ].join(' ')
          } else {
            pathData = [
              `M ${centerX} ${centerY}`,
              `L ${x1} ${y1}`,
              `A ${radius} ${radius} 0 ${largeArcFlag} 1 ${x2} ${y2}`,
              'Z'
            ].join(' ')
          }
          
          const result = (
            <path
              key={`${item.name}-${index}`}
              d={pathData}
              fill={colors[index % colors.length]}
              stroke="white"
              strokeWidth="2"
            />
          )
          
          currentAngle += angle
          return result
        })}
      </svg>
      <div className="pie-chart-legend">
        {chartData.map((item, index) => (
          <div key={`legend-${item.name}-${index}`} className="legend-item">
            <div 
              className="legend-color" 
              style={{ backgroundColor: colors[index % colors.length] }}
            ></div>
            <span>{item.name}: {item.value}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

// Helper function to render graph view
const renderGraphView = (data: any, cellId?: string, lastExecuted?: string): React.ReactNode => {
  if (!data || typeof data !== 'object') {
    return <div className="empty-graph">No graph data available</div>
  }

  // Check if data already has graph structure
  if (data.graph && data.graph.nodes && data.graph.links) {
    return <DebugForceGraph 
      graphData={data.graph} 
      cellId={cellId || 'unknown'} 
    />
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
    
    return <DebugForceGraph 
      graphData={{ nodes, links }} 
      cellId={cellId || 'unknown'} 
    />
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
      return <DebugForceGraph 
        graphData={{ nodes, links }} 
        cellId={cellId || 'unknown'} 
      />
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

// Sortable Table Component with persistence
const SortableTable = memo(({ data, graph, query, cellId }: {
  data: any,
  graph?: any,
  query?: string,
  cellId: string
}) => {
  const [sortColumn, setSortColumn] = useState<string | null>(null)
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('desc')

  // Load saved sort state on mount
  useEffect(() => {
    const savedSort = localStorage.getItem(`table-sort-${cellId}`)
    if (savedSort) {
      try {
        const { column, direction } = JSON.parse(savedSort)
        setSortColumn(column)
        setSortDirection(direction)
      } catch (e) {
        // Ignore invalid saved state
      }
    }
  }, [cellId])

  // Save sort state when it changes
  useEffect(() => {
    if (sortColumn) {
      localStorage.setItem(`table-sort-${cellId}`, JSON.stringify({
        column: sortColumn,
        direction: sortDirection
      }))
    }
  }, [sortColumn, sortDirection, cellId])

  const handleSort = (column: string) => {
    if (sortColumn === column) {
      // Same column: toggle direction
      setSortDirection(sortDirection === 'desc' ? 'asc' : 'desc')
    } else {
      // New column: start with desc
      setSortColumn(column)
      setSortDirection('desc')
    }
  }
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
  
  // Sort rows based on current sort settings
  const sortedRows = React.useMemo(() => {
    if (!sortColumn) return rows

    return [...rows].sort((a, b) => {
      const aVal = a[sortColumn]
      const bVal = b[sortColumn]
      
      // Handle null/undefined values
      if (aVal == null && bVal == null) return 0
      if (aVal == null) return sortDirection === 'asc' ? -1 : 1
      if (bVal == null) return sortDirection === 'asc' ? 1 : -1
      
      // Convert to strings for comparison
      const aStr = String(aVal).toLowerCase()
      const bStr = String(bVal).toLowerCase()
      
      // Try numeric comparison first
      const aNum = parseFloat(aStr)
      const bNum = parseFloat(bStr)
      if (!isNaN(aNum) && !isNaN(bNum)) {
        return sortDirection === 'asc' ? aNum - bNum : bNum - aNum
      }
      
      // String comparison
      if (aStr < bStr) return sortDirection === 'asc' ? -1 : 1
      if (aStr > bStr) return sortDirection === 'asc' ? 1 : -1
      return 0
    })
  }, [rows, sortColumn, sortDirection])

  const getSortIcon = (column: string) => {
    if (sortColumn !== column) return null
    return sortDirection === 'desc' ? <ChevronDown size={14} /> : <ChevronUp size={14} />
  }

  return (
    <table className="results-table">
      <thead>
        <tr>
          {columns.map(col => (
            <th 
              key={col} 
              onClick={() => handleSort(col)}
              style={{ 
                cursor: 'pointer', 
                userSelect: 'none'
              }}
              title={`Sort by ${col}`}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                {getSortIcon(col) && (
                  <span>
                    {getSortIcon(col)}
                  </span>
                )}
                <span>{col}</span>
              </div>
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {sortedRows.map((row, index) => (
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
})

// Simple wrapper function that creates the sortable table
const renderTableView = (data: any, graph?: any, query?: string, cellId?: string): React.ReactNode => {
  return <SortableTable data={data} graph={graph} query={query} cellId={cellId || 'unknown'} />
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

const CellComponentImpl: React.FC<CellComponentProps> = ({
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
  const [pollingIntervalId, setPollingIntervalId] = useState<NodeJS.Timeout | null>(null)
  const [selectedInterval, setSelectedInterval] = useState(cell.refresh_interval || 5000)
  const [dropdownOpen, setDropdownOpen] = useState(false)
  const [isEditingName, setIsEditingName] = useState(false)
  const [cellName, setCellName] = useState(cell.name || '')

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollingIntervalId) {
        clearInterval(pollingIntervalId)
        clearTimeout(pollingIntervalId as any)
      }
    }
  }, [pollingIntervalId])

  // Check if query contains mutation keywords
  const isMutationQuery = () => {
    if (!cell.query) return false
    const query = cell.query.toLowerCase()
    return query.includes('create') || query.includes('set') || query.includes('delete')
  }

  const isPollingDisabled = isMutationQuery()

  // Start polling if cell has refresh_interval set and no error  
  useEffect(() => {
    // Only start polling on initial mount if conditions are met
    if (cell.refresh_interval && cell.refresh_interval > 0 && !pollingIntervalId && !cell.error && !isPollingDisabled) {
      setPollingIntervalId(1 as any) // Set dummy ID to indicate polling is active
      executePoll()
    }
    // Note: This intentionally has limited dependencies to avoid restart loops
  }, []) // Only run on mount, other effects handle state changes

  // Stop polling if query becomes a mutation query
  useEffect(() => {
    if (isPollingDisabled && pollingIntervalId) {
      clearInterval(pollingIntervalId)
      clearTimeout(pollingIntervalId as any)
      setPollingIntervalId(null)
    }
  }, [isPollingDisabled, pollingIntervalId])

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownOpen) {
        const target = event.target as Element
        if (!target.closest('.run-button-dropdown')) {
          setDropdownOpen(false)
        }
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [dropdownOpen])

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

  const handleSaveName = async () => {
    try {
      const response = await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ name: cellName }),
      })
      
      if (response.ok) {
        onUpdate(cell.id, { name: cellName })
        setIsEditingName(false)
      }
    } catch (error) {
      console.error('Failed to save cell name:', error)
    }
  }

  const handleCancelName = () => {
    setCellName(cell.name || '')
    setIsEditingName(false)
  }

  const executeQuery = async (): Promise<boolean> => {
    try {
      // For polling, don't show running state to reduce flicker
      if (!pollingIntervalId) {
        onUpdate(cell.id, { is_running: true })
      }
      
      const response = await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}/execute`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          context: cell.config?.context,
          namespace: cell.config?.namespace,
        }),
      })
      
      if (response.ok) {
        const responseData = await response.json()
        // Extract the actual result data if it's wrapped
        const result = responseData.result || responseData
        onUpdate(cell.id, { 
          results: result, 
          is_running: false,
          last_executed: new Date().toISOString(),
          error: undefined
        })
        return true // Success
      } else {
        onUpdate(cell.id, { 
          is_running: false,
          error: 'Failed to execute query'
        })
        return false // Error
      }
    } catch (error) {
      onUpdate(cell.id, { 
        is_running: false,
        error: 'Network error'
      })
      return false // Error
    }
  }

  const executePoll = async () => {
    const success = await executeQuery()
    
    if (!success) {
      // Stop polling on error
      if (pollingIntervalId) {
        clearInterval(pollingIntervalId)
        clearTimeout(pollingIntervalId as any)
        setPollingIntervalId(null)
      }
      return
    }
    
    // Schedule next execution after successful completion
    // Use callback to ensure we check the latest state
    setPollingIntervalId(prevId => {
      if (prevId) {
        // Still polling, schedule next execution
        return setTimeout(executePoll, selectedInterval) as any
      }
      return null // Polling was stopped
    })
  }

  const handleRunPause = async () => {
    if (pollingIntervalId) {
      // Stop polling
      clearInterval(pollingIntervalId)
      clearTimeout(pollingIntervalId as any)
      setPollingIntervalId(null)
      
      // Update refresh_interval to 0 to indicate no polling
      await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_interval: 0 }),
      })
      onUpdate(cell.id, { refresh_interval: 0 })
    } else {
      // Start polling with timeout-based approach
      // Set a dummy ID to indicate polling is active
      setPollingIntervalId(1 as any)
      executePoll()
      
      // Update refresh_interval in backend
      await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_interval: selectedInterval }),
      })
      onUpdate(cell.id, { refresh_interval: selectedInterval })
    }
  }

  const handleIntervalChange = async (newInterval: number) => {
    setSelectedInterval(newInterval)
    
    // If currently polling, restart with new interval
    if (pollingIntervalId) {
      clearInterval(pollingIntervalId)
      clearTimeout(pollingIntervalId as any)
      setPollingIntervalId(null)
      
      // Restart polling with new interval
      setPollingIntervalId(1 as any) // Set dummy ID to indicate polling is active
      executePoll()
      
      // Update in backend
      await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_interval: newInterval }),
      })
      onUpdate(cell.id, { refresh_interval: newInterval })
    }
  }

  const intervalOptions = [
    { value: 1000, label: '1s' },
    { value: 5000, label: '5s' },
    { value: 15000, label: '15s' },
    { value: 30000, label: '30s' },
    { value: 60000, label: '1m' }
  ]

  const getIntervalLabel = (value: number) => {
    return intervalOptions.find(opt => opt.value === value)?.label || '5s'
  }

  const handleIntervalSelect = (newInterval: number) => {
    setDropdownOpen(false)
    handleIntervalChange(newInterval)
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

  // Sync state with cell data when cell changes (fixes persistence on reload)
  useEffect(() => {
    const newMode = getVisualizationMode()
    const newDocMode = getDocumentMode()
    const newGraphMode = getGraphMode()
    
    setCurrentMode(newMode)
    setDocumentMode(newDocMode) 
    setGraphMode(newGraphMode)
  }, [cell.visualization_type, cell.config])

  // Sync polling interval state with cell data
  useEffect(() => {
    setSelectedInterval(cell.refresh_interval || 5000)
  }, [cell.refresh_interval])

  // Sync cell name state with cell data
  useEffect(() => {
    setCellName(cell.name || '')
  }, [cell.name])

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
      className={`cell ${isEditing ? 'editing' : getExecutionStatus()} ${pollingIntervalId ? 'polling' : ''} ${isDragging ? 'dragging' : ''} ${isDragOver ? 'drag-over' : ''}`}
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
            {isEditingName ? (
              <input
                type="text"
                value={cellName}
                onChange={(e) => setCellName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    handleSaveName()
                  } else if (e.key === 'Escape') {
                    handleCancelName()
                  }
                }}
                onBlur={handleSaveName}
                autoFocus
                className="cell-name-editor"
                placeholder="Cell name"
              />
            ) : (
              <span 
                className="cell-name-display"
                onClick={() => setIsEditingName(true)}
                title="Click to edit cell name"
              >
                {cellName || 'Query'}
              </span>
            )}
          </span>
          <ContextSelector
            context={cell.config?.context}
            namespace={cell.config?.namespace}
            onContextChange={(context, namespace) => {
              onUpdate(cell.id, {
                config: {
                  ...cell.config,
                  context,
                  namespace
                }
              })
            }}
            className="compact"
          />
          {cell.last_executed && (
            <span className="cell-executed">
              Last executed: {new Date(cell.last_executed).toLocaleTimeString()}
            </span>
          )}
          {pollingIntervalId && (
            <span className="cell-polling-indicator">
              Live
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
              <div className="run-button-group">
                <div className="run-button-dropdown">
                  <button 
                    onClick={() => !isPollingDisabled && setDropdownOpen(!dropdownOpen)}
                    className={`cell-action execute run-button-trigger ${isPollingDisabled ? 'disabled' : ''}`}
                    disabled={isPollingDisabled}
                    title={isPollingDisabled ? 'Polling disabled for mutation queries (CREATE, SET, DELETE)' : 'Select polling interval'}
                  >
                    <span className="interval-label">{getIntervalLabel(selectedInterval)}</span>
                    <ChevronDown size={12} />
                  </button>
                  {dropdownOpen && (
                    <div className="interval-dropdown">
                      {intervalOptions.map(option => (
                        <button
                          key={option.value}
                          onClick={() => handleIntervalSelect(option.value)}
                          className={`interval-option ${selectedInterval === option.value ? 'active' : ''}`}
                        >
                          {option.label}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
                <button 
                  onClick={isPollingDisabled ? () => executeQuery() : handleRunPause} 
                  disabled={cell.is_running && !pollingIntervalId}
                  className="cell-action execute run-button-main"
                >
                  {pollingIntervalId && !isPollingDisabled ? (
                    <>
                      <Pause size={14} />
                      Pause
                    </>
                  ) : (
                    <>
                      <Play size={14} />
                      {cell.is_running ? 'Running...' : 'Run'}
                    </>
                  )}
                </button>
              </div>
              <button onClick={() => setIsEditing(true)} className="cell-action edit">
                <Edit3 size={14} />
                Edit
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
                {renderTableView(cell.results?.data || cell.results, cell.results?.graph, cell.query, cell.id)}
              </div>
            )}
            {currentMode === 'graph' && (
              <div className="graph-output">
                {graphMode === 'force' && renderGraphView(cell.results, cell.id, cell.last_executed)}
                {graphMode === 'pie' && renderPieChartView(cell.results, cell.query)}
                {graphMode === 'tree' && (
                  <div className="tree-chart-placeholder">
                    <p>Tree visualization coming soon...</p>
                    <div className="fallback-view">
                      {renderGraphView(cell.results, cell.id, cell.last_executed)}
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

// Memoized export to prevent unnecessary re-renders
export const CellComponent = memo(CellComponentImpl, (prevProps, nextProps) => {
  // Only re-render if the specific cell or its state has actually changed
  return (
    prevProps.cell.id === nextProps.cell.id &&
    prevProps.cell.last_executed === nextProps.cell.last_executed &&
    prevProps.cell.is_running === nextProps.cell.is_running &&
    JSON.stringify(prevProps.cell.results) === JSON.stringify(nextProps.cell.results) &&
    JSON.stringify(prevProps.cell.config) === JSON.stringify(nextProps.cell.config) &&
    prevProps.cell.query === nextProps.cell.query &&
    prevProps.cell.error === nextProps.cell.error &&
    prevProps.isDragging === nextProps.isDragging &&
    prevProps.isDragOver === nextProps.isDragOver
  )
})