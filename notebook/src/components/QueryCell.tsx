import React, { useState, useCallback, useEffect, memo, useRef, useMemo } from 'react'

// Debounce function
function debounce<F extends (...args: any[]) => any>(func: F, wait: number): (...args: Parameters<F>) => void {
  let timeoutId: ReturnType<typeof setTimeout> | null = null
  return (...args: Parameters<F>) => {
    if (timeoutId !== null) {
      clearTimeout(timeoutId)
    }
    timeoutId = setTimeout(() => func(...args), wait)
  }
}
import { Cell, VisualizationType, VisualizationMode, DocumentMode, GraphMode } from '../types/notebook'
import ForceGraph2D from 'react-force-graph-2d'
import * as jsYaml from 'js-yaml'
import { FileText, Table, Network, Edit3, Play, Pause, Save, X, Trash2, Search, ChevronDown, ChevronUp, ScrollText, Square, Filter, Clock } from 'lucide-react'
import { SyntaxHighlighter } from './SyntaxHighlighter'
import { ContextSelector } from './ContextSelector'
import { CyphernetesEditor } from './CyphernetesEditor'
import { buildK8sResourceMap, findGraphPatterns, buildTableRows } from '../utils/tableGrouping'
import { Prism as PrismSyntaxHighlighter } from 'react-syntax-highlighter'
import { oneLight } from 'react-syntax-highlighter/dist/cjs/styles/prism'
import './CellComponent.css'

// Kubernetes resource type color mapping
const K8S_RESOURCE_COLORS: {[key: string]: string} = {
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

// Resource type abbreviations
const getResourceAbbreviation = (resourceType: string): string => {
  const abbreviations: {[key: string]: string} = {
    'Pod': 'P',
    'Service': 'S',
    'Deployment': 'D',
    'StatefulSet': 'SS',
    'ConfigMap': 'CM',
    'Secret': 'SEC',
    'PersistentVolumeClaim': 'PVC',
    'Ingress': 'ING',
    'Job': 'J',
    'CronJob': 'CJ',
    'Namespace': 'NS',
    'ReplicaSet': 'RS',
    'DaemonSet': 'DS',
    'Endpoint': 'EP',
    'Node': 'N',
    'PodDisruptionBudget': 'PDB'
  }
  return abbreviations[resourceType] || resourceType.substring(0, 2).toUpperCase()
}

// Simple ForceGraph wrapper with dynamic sizing
const DebugForceGraph = memo(({ graphData, cellId }: { 
  graphData: any, 
  cellId: string
}) => {
  const containerRef = useRef<HTMLDivElement>(null)
  const [dimensions, setDimensions] = useState({ width: 800, height: 368 })
  
  useEffect(() => {
    const updateDimensions = () => {
      if (containerRef.current) {
        const rect = containerRef.current.getBoundingClientRect()
        setDimensions({
          width: rect.width || 800,
          height: 368 // Keep fixed height
        })
      }
    }
    
    updateDimensions()
    window.addEventListener('resize', updateDimensions)
    
    return () => window.removeEventListener('resize', updateDimensions)
  }, [])
  

  // Custom node label with resource kind (like web client)
  const getNodeLabel = (node: any) => {
    const resourceType = node.kind || node.type || 'Unknown'
    const name = node.name || 'unnamed'
    return `${resourceType.toLowerCase()}/${name}`
  }

  // Custom canvas object for node with text  
  const drawNodeWithText = (node: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
    const nodeRadius = Math.max(6, 12 / globalScale)
    
    // Use kind from graph structure (like web client)
    const resourceType = node.kind || node.type || 'unknown'
    const label = getResourceAbbreviation(resourceType)
    const fontSize = Math.max(6, 10 / globalScale)
    
    // Draw the circle
    ctx.beginPath()
    ctx.arc(node.x, node.y, nodeRadius, 0, 2 * Math.PI)
    ctx.fillStyle = K8S_RESOURCE_COLORS[resourceType] || '#aaaaaa'
    ctx.fill()
    
    // Draw the text
    ctx.font = `bold ${fontSize}px Arial`
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillStyle = '#ffffff'
    ctx.strokeStyle = '#000000'
    ctx.lineWidth = 0.5
    
    ctx.strokeText(label, node.x, node.y)
    ctx.fillText(label, node.x, node.y)
  }
  
  return (
    <div ref={containerRef} style={{ width: '100%', height: '368px' }}>
      <ForceGraph2D
        key={`force-${cellId}`}
        graphData={graphData}
        nodeLabel={getNodeLabel}
        nodeColor={(node: any) => K8S_RESOURCE_COLORS[node.kind || node.type] || '#aaaaaa'}
        linkColor={() => '#999'}
        linkWidth={2}
        nodeRelSize={12}
        enableZoomInteraction={true}
        enableNodeDrag={true}
        width={dimensions.width}
        height={dimensions.height}
        nodeCanvasObject={drawNodeWithText}
        nodeCanvasObjectMode={() => "replace"}
      />
    </div>
  )
})


// Force graph legend component
const ForceGraphLegend: React.FC<{ visibleTypes: Set<string> }> = ({ visibleTypes }) => {
  // Only show legend items for resource types that are actually present in the graph
  const relevantTypes = Array.from(visibleTypes).filter(type => K8S_RESOURCE_COLORS[type])
  
  if (relevantTypes.length === 0) {
    return null
  }

  return (
    <div className="force-graph-legend">
      <div className="legend-items">
        {relevantTypes.sort().map(type => (
          <div key={type} className="legend-item">
            <div 
              className="legend-color-box"
              style={{ backgroundColor: K8S_RESOURCE_COLORS[type] }}
            />
            <span className="legend-label">{type}</span>
          </div>
        ))}
        {visibleTypes.has('unknown') && (
          <div key="unknown" className="legend-item">
            <div 
              className="legend-color-box"
              style={{ backgroundColor: '#aaaaaa' }}
            />
            <span className="legend-label">Unknown</span>
          </div>
        )}
      </div>
    </div>
  )
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

// Helper function to parse Kubernetes resource notation to numbers
const parseK8sValue = (value: string): number => {
  if (typeof value !== 'string') return parseFloat(String(value)) || 0
  
  const trimmed = value.trim().toLowerCase()
  
  // Handle CPU units (millicores)
  if (trimmed.endsWith('m')) {
    const num = parseFloat(trimmed.slice(0, -1))
    return isNaN(num) ? 0 : num / 1000 // Convert millicores to cores
  }
  
  // Handle memory units
  const memoryUnits: {[key: string]: number} = {
    'ki': 1024,
    'mi': 1024 * 1024,
    'gi': 1024 * 1024 * 1024,
    'ti': 1024 * 1024 * 1024 * 1024,
    'k': 1000,
    'm': 1000 * 1000,
    'g': 1000 * 1000 * 1000,
    't': 1000 * 1000 * 1000 * 1000,
  }
  
  for (const [suffix, multiplier] of Object.entries(memoryUnits)) {
    if (trimmed.endsWith(suffix)) {
      const num = parseFloat(trimmed.slice(0, -suffix.length))
      return isNaN(num) ? 0 : num * multiplier
    }
  }
  
  // Handle plain numbers
  const num = parseFloat(trimmed)
  return isNaN(num) ? 0 : num
}

// Helper function to format display values
const formatDisplayValue = (originalValue: string | number, parsedValue: number): string => {
  if (typeof originalValue === 'string') {
    const trimmed = originalValue.trim().toLowerCase()
    if (trimmed.endsWith('m')) {
      // For CPU, show cores with appropriate precision
      if (parsedValue < 1) {
        return `${originalValue} (${parsedValue.toFixed(3)} cores)`
      } else {
        return `${originalValue} (${parsedValue.toFixed(1)} cores)`
      }
    }
    if (trimmed.match(/[kmgt]i?$/)) return `${originalValue} (${(parsedValue / (1024 * 1024)).toFixed(2)}Mi)`
  }
  return String(originalValue)
}

// Helper function to render bar chart view
const renderBarChartView = (data: any, query?: string): React.ReactNode => {
  if (!data || typeof data !== 'object') {
    return <div className="empty-bar">No data available for bar chart</div>
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
      <div className="bar-chart-error">
        <p>Bar charts are only supported for queries with a single return element.</p>
        <p>Your query returns {requestedFields.length} elements: {requestedFields.join(', ')}</p>
      </div>
    )
  }

  // Extract data for bar chart
  const dataToProcess = data.data || data
  if (!dataToProcess || typeof dataToProcess !== 'object') {
    return <div className="empty-bar">No data structure suitable for bar chart</div>
  }

  // Get the single variable data
  const entries = Object.entries(dataToProcess)
  if (entries.length === 0) {
    return <div className="empty-bar">No data to display in bar chart</div>
  }

  // Use the first (and ideally only) variable's data
  const [variableName, resources] = entries[0]
  if (!Array.isArray(resources)) {
    return <div className="empty-bar">Data is not in array format suitable for bar chart</div>
  }

  // Extract the field path from the return clause to use for values
  let valueField: string | null = null
  if (requestedFields.length === 1) {
    const field = requestedFields[0]
    // Extract the property path after the variable (e.g., "p.status.phase" -> "status.phase")
    const dotIndex = field.indexOf('.')
    if (dotIndex !== -1) {
      valueField = field.substring(dotIndex + 1)
    }
  }

  // Helper function to extract nested field value
  const extractNestedValue = (obj: any, path: string): any => {
    const parts = path.split('.')
    let current = obj
    
    for (const part of parts) {
      if (!current || typeof current !== 'object') return null
      current = current[part]
    }
    
    return current
  }

  // Extract values for bar chart
  const chartData: Array<{name: string, value: number, displayValue: string, originalValue: any}> = []
  
  resources.forEach((resource: any, index: number) => {
    if (typeof resource === 'object' && resource !== null) {
      let name = 'Unknown'
      let originalValue: any = null
      
      // Get the name for the bar
      if (typeof resource.name === 'string') {
        name = resource.name
      } else if (typeof resource.metadata?.name === 'string') {
        name = resource.metadata.name
      } else {
        name = `Item ${index + 1}`
      }
      
      if (valueField) {
        // Use the specific field requested in the return clause
        originalValue = extractNestedValue(resource, valueField)
      } else {
        // Fallback: look for common numeric fields
        const numericFields = ['value', 'count', 'size', 'capacity', 'requests', 'limits']
        for (const field of numericFields) {
          if (resource[field] !== undefined) {
            originalValue = resource[field]
            break
          }
        }
      }
      
      if (originalValue !== null && originalValue !== undefined) {
        const parsedValue = parseK8sValue(String(originalValue))
        if (parsedValue > 0) { // Only include positive values
          chartData.push({
            name,
            value: parsedValue,
            displayValue: formatDisplayValue(originalValue, parsedValue),
            originalValue
          })
        }
      }
    }
  })

  if (chartData.length === 0) {
    return <div className="empty-bar">No numeric data found for bar chart</div>
  }

  // Sort by value descending
  chartData.sort((a, b) => b.value - a.value)

  // Calculate chart dimensions
  const maxValue = Math.max(...chartData.map(item => item.value))

  const colors = ['#4285F4', '#34A853', '#FBBC05', '#EA4335', '#8E44AD', '#F39C12', '#1ABC9C', '#E74C3C']

  return (
    <div className="bar-chart-container">
      <div className="bar-chart-header">
        <h4>Values Comparison</h4>
        {valueField && <p className="field-info">Showing: {valueField}</p>}
      </div>
      <div className="bar-chart-content">
        {chartData.map((item, index) => {
          const barWidthPercent = maxValue > 0 ? (item.value / maxValue) * 100 : 0
          
          return (
            <div key={`${item.name}-${index}`} className="bar-item">
              <div className="bar-label">
                <span title={item.name}>{item.name}</span>
              </div>
              <div className="bar-container">
                <div 
                  className="bar-fill"
                  style={{ 
                    width: `${barWidthPercent}%`,
                    backgroundColor: colors[index % colors.length]
                  }}
                  title={`${item.name}: ${item.displayValue}`}
                />
              </div>
              <div className="bar-value">
                <span title={item.displayValue}>{item.displayValue}</span>
              </div>
            </div>
          )
        })}
      </div>
      <div className="bar-chart-legend">
        <p>Total items: {chartData.length}</p>
        <p>Max value: {formatDisplayValue(chartData[0]?.originalValue, maxValue)}</p>
      </div>
    </div>
  )
}

// Helper function to render graph view
const renderGraphView = (data: any, cellId?: string, lastExecuted?: string): React.ReactNode => {
  if (!data || typeof data !== 'object') {
    return <div className="empty-graph">No graph data available</div>
  }

  // Use the graph data if available (EXACTLY like web client)  
  const graphData = data.graph || data.Graph
  
  // Check for lowercase nodes/links first (already processed format)
  if (graphData && graphData.nodes && graphData.links) {
    const visibleTypes = new Set<string>()
    graphData.nodes.forEach((node: any) => {
      const nodeType = node.kind || node.type || 'unknown'
      visibleTypes.add(nodeType)
    })
    
    return (
      <div className="force-graph-container">
        <DebugForceGraph 
          graphData={graphData} 
          cellId={cellId || 'unknown'} 
        />
        <ForceGraphLegend visibleTypes={visibleTypes} />
      </div>
    )
  }
  
  // Check for uppercase Nodes/Edges (raw Cyphernetes format)
  if (graphData && graphData.Nodes && 'Edges' in graphData) {
    
    // Process exactly like web client: use node.Kind directly from graph with deduplication
    const nodesMap = new Map()
    graphData.Nodes.forEach((node: any) => {
      const nodeId = `${node.Kind}/${node.Name}`
      
      // Deduplicate nodes by ID
      if (!nodesMap.has(nodeId)) {
        nodesMap.set(nodeId, {
          id: nodeId,
          dataRefId: node.Id,
          kind: node.Kind,        // Resource type from graph structure
          name: node.Name,
          type: node.Kind,        // For backward compatibility
          namespace: node.Namespace || 'default',
          ...node
        })
      }
    })
    
    const nodes = Array.from(nodesMap.values())
    
    const links = (graphData.Edges || []).map((edge: any) => ({
      source: edge.From || edge.source,
      target: edge.To || edge.target,
      type: edge.Type || edge.type || 'relationship'
    }))
    
    const visibleTypes = new Set<string>()
    nodes.forEach((node: any) => {
      visibleTypes.add(node.kind) // Use kind from graph
    })
    
    return (
      <div className="force-graph-container">
        <DebugForceGraph 
          graphData={{ nodes, links }} 
          cellId={cellId || 'unknown'} 
        />
        <ForceGraphLegend visibleTypes={visibleTypes} />
      </div>
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
      const visibleTypes = new Set<string>()
      nodes.forEach((node: any) => {
        const nodeType = node.type || 'unknown'
        visibleTypes.add(nodeType)
      })
      
      return (
        <div className="force-graph-container">
          <DebugForceGraph 
            graphData={{ nodes, links }} 
            cellId={cellId || 'unknown'} 
          />
          <ForceGraphLegend visibleTypes={visibleTypes} />
        </div>
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
    // Build resource mapping and find patterns
    const k8sResourceMap = buildK8sResourceMap(resourcesByVariable, nodes || [])
    const patterns = findGraphPatterns(k8sResourceMap, edges)
    
    // Build rows from patterns
    const patternRows = buildTableRows(patterns, columns, resourcesByVariable)
    rows.push(...patternRows)
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

interface QueryCellProps {
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

export const QueryCell: React.FC<QueryCellProps> = ({
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
  const [isEditing, setIsEditing] = useState(false)
  const [query, setQuery] = useState(cell.query)
  const [isPollingActive, setIsPollingActive] = useState(false)
  const isPollingActiveRef = useRef(false)
  const [selectedInterval, setSelectedInterval] = useState(cell.refresh_interval || 5000)
  const selectedIntervalRef = useRef(cell.refresh_interval || 5000)
  const [dropdownOpen, setDropdownOpen] = useState(false)
  const [isEditingName, setIsEditingName] = useState(false)
  const [cellName, setCellName] = useState(cell.name || '')
  
  // Local polling results to prevent parent re-renders
  const [localResults, setLocalResults] = useState<any>(cell.results)
  const [localError, setLocalError] = useState<string | undefined>(cell.error)
  const [localLastExecuted, setLocalLastExecuted] = useState<string | undefined>(cell.last_executed)
  const [localIsRunning, setLocalIsRunning] = useState<boolean>(cell.is_running || false)
  
  // Autocompletion state
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [cursorPosition, setCursorPosition] = useState(0)
  const [selectedSuggestionIndex, setSelectedSuggestionIndex] = useState(-1)
  const [suggestionsPosition, setSuggestionsPosition] = useState({ top: 0, left: 0 })
  
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const suggestionsRef = useRef<HTMLDivElement>(null)
  
  // Refs for cleanup
  const abortControllerRef = useRef<AbortController | null>(null)
  const isMountedRef = useRef(true)
  const pollingTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  
  // Helper to update both state and ref
  const setPollingActiveState = (active: boolean) => {
    setIsPollingActive(active)
    isPollingActiveRef.current = active
  }
  
  // Helper to update both interval state and ref
  const setIntervalState = (interval: number) => {
    setSelectedInterval(interval)
    selectedIntervalRef.current = interval
  }

  // Set mounted flag and cleanup on unmount
  useEffect(() => {
    isMountedRef.current = true
    
    return () => {
      isMountedRef.current = false
      
      // Cancel any in-flight requests
      if (abortControllerRef.current) {
        abortControllerRef.current.abort()
        abortControllerRef.current = null
      }
      
      // Clear polling timeout
      if (pollingTimeoutRef.current) {
        clearTimeout(pollingTimeoutRef.current)
        pollingTimeoutRef.current = null
      }
    }
  }, [])
  
  // Sync local state with cell props when they change (for manual updates)
  useEffect(() => {
    setLocalResults(cell.results)
  }, [cell.results])
  
  useEffect(() => {
    setLocalError(cell.error)
  }, [cell.error])
  
  useEffect(() => {
    setLocalLastExecuted(cell.last_executed)
  }, [cell.last_executed])
  
  useEffect(() => {
    setLocalIsRunning(cell.is_running || false)
  }, [cell.is_running])


  // Start polling if cell has refresh_interval set  
  useEffect(() => {
    // Prevent multiple polling starts
    const shouldStartPolling = 
      cell.refresh_interval && 
      cell.refresh_interval > 0 && 
      !isPollingDisabled && 
      isMountedRef.current
    
    if (shouldStartPolling) {
      setPollingActiveState(true)
      // Use a small delay to prevent racing with other effects
      const startTimeout = setTimeout(() => {
        if (isMountedRef.current && !pollingTimeoutRef.current) {
          executePoll()
        }
      }, 100)
      
      return () => {
        clearTimeout(startTimeout)
      }
    }
  }, []) // Only run on mount


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
        
        // Execute the query to provide immediate feedback after saving
        await executeQuery()
        
        // If we're in logs mode and currently streaming, restart streaming with new query
        if (currentMode === 'logs' && logsStreaming) {
          // Stop current stream
          if (logsEventSourceRef.current) {
            logsEventSourceRef.current.close()
            logsEventSourceRef.current = null
          }
          // Mark that we're manually restarting to prevent auto-start interference
          isManuallyRestartingRef.current = true
          setLogsStreaming(false)
          
          // Wait for pod extraction and then restart
          setTimeout(() => {
            if (currentMode === 'logs') {
              isManuallyRestartingRef.current = false
              setLogsStreaming(true)
            }
          }, 500)
        }
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
    // Cancel any previous request
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
    }
    
    // Create new abort controller for this request
    abortControllerRef.current = new AbortController()
    
    try {
      // Update running state
      if (isMountedRef.current) {
        if (isPollingActiveRef.current) {
          // For polling, use local state only
          setLocalIsRunning(true)
        } else {
          // For manual runs, update parent
          onUpdate(cell.id, { is_running: true })
          setLocalIsRunning(true)
        }
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
        signal: abortControllerRef.current.signal
      })
      
      if (!isMountedRef.current) return false
      
      if (response.ok) {
        const responseData = await response.json()
        
        // Check if the response contains an error even with 200 status
        // Error can be at top level or nested in result
        const errorMsg = responseData.error || responseData.result?.error
        if (errorMsg) {
          if (isMountedRef.current) {
            // For polling updates, use local state
            if (isPollingActiveRef.current) {
              setLocalError(errorMsg)
              setLocalIsRunning(false)
              // No parent updates during polling!
            } else {
              // For manual runs, update parent
              onUpdate(cell.id, { 
                is_running: false,
                error: errorMsg
              })
              setLocalError(errorMsg)
              setLocalIsRunning(false)
            }
          }
          return false // Error
        }
        
        // Extract the actual result data if it's wrapped
        const result = responseData.result || responseData
        const timestamp = new Date().toISOString()
        
        if (isMountedRef.current) {
          // For polling updates, use local state to prevent parent re-renders
          if (isPollingActiveRef.current) {
            setLocalResults(result)
            setLocalError(undefined)
            setLocalLastExecuted(timestamp)
            setLocalIsRunning(false)
            // No parent updates during polling!
          } else {
            // For manual runs, update parent with all data
            onUpdate(cell.id, { 
              results: result, 
              is_running: false,
              last_executed: timestamp,
              error: undefined
            })
            // Also update local state to stay in sync
            setLocalResults(result)
            setLocalError(undefined)
            setLocalLastExecuted(timestamp)
            setLocalIsRunning(false)
          }
        }
        return true // Success
      } else {
        if (isMountedRef.current) {
          // Try to parse the actual error message from the server
          let errorMsg = 'Failed to execute query'
          try {
            const errorResponse = await response.json()
            errorMsg = errorResponse.error || errorResponse.message || errorMsg
          } catch (e) {
            // If we can't parse the error, use the default message
          }
          
          // For polling updates, use local state
          if (isPollingActiveRef.current) {
            setLocalError(errorMsg)
            setLocalIsRunning(false)
            // No parent updates during polling!
          } else {
            // For manual runs, update parent
            onUpdate(cell.id, { 
              is_running: false,
              error: errorMsg
            })
            setLocalError(errorMsg)
            setLocalIsRunning(false)
          }
        }
        return false // Error
      }
    } catch (error: any) {
      // Ignore abort errors
      if (error.name === 'AbortError') {
        return false
      }
      
      if (isMountedRef.current) {
        const errorMsg = 'Network error'
        // For polling updates, use local state
        if (isPollingActiveRef.current) {
          setLocalError(errorMsg)
          setLocalIsRunning(false)
          // No parent updates during polling!
        } else {
          // For manual runs, update parent
          onUpdate(cell.id, { 
            is_running: false,
            error: errorMsg
          })
          setLocalError(errorMsg)
          setLocalIsRunning(false)
        }
      }
      return false // Error
    } finally {
      // Clear the abort controller reference
      if (abortControllerRef.current?.signal.aborted) {
        abortControllerRef.current = null
      }
    }
  }

  const executePoll = async () => {
    if (!isMountedRef.current) return
    
    const success = await executeQuery()
    
    if (!isMountedRef.current) return
    
    if (!success) {
      // Stop polling on error
      setPollingActiveState(false)
      if (pollingTimeoutRef.current) {
        clearTimeout(pollingTimeoutRef.current)
        pollingTimeoutRef.current = null
      }
      return
    }
    
    // Only schedule next execution if still mounted and polling is active
    if (isMountedRef.current && isPollingActiveRef.current) {
      pollingTimeoutRef.current = setTimeout(() => {
        if (isMountedRef.current && isPollingActiveRef.current) {
          executePoll()
        }
      }, selectedIntervalRef.current)
    }
  }


  const handleRunPause = async () => {
    // In logs mode, handle streaming instead of polling
    if (currentMode === 'logs') {
      if (logsStreaming) {
        // Stop logs streaming - set flag to prevent auto-restart
        hasAutoStartedRef.current = true
        if (logsEventSourceRef.current) {
          logsEventSourceRef.current.close()
          logsEventSourceRef.current = null
        }
        updateLogsStreamingState(false)
      } else {
        // Start logs streaming - reset auto-start flag
        hasAutoStartedRef.current = false
        startLogsStreaming()
      }
      return
    }

    // If polling is disabled (mutating query), execute once instead of polling
    if (isPollingDisabled) {
      await executeQuery()
      return
    }

    if (isPollingActiveRef.current) {
      // Stop polling
      setPollingActiveState(false)
      
      // Clear polling timeout
      if (pollingTimeoutRef.current) {
        clearTimeout(pollingTimeoutRef.current)
        pollingTimeoutRef.current = null
      }
      
      // Cancel any in-flight requests
      if (abortControllerRef.current) {
        abortControllerRef.current.abort()
        abortControllerRef.current = null
      }
      
      // Update refresh_interval to 0 to indicate no polling
      if (isMountedRef.current) {
        await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_interval: 0 }),
        })
        onUpdate(cell.id, { refresh_interval: 0 })
      }
    } else {
      // Test query first before starting polling
      const success = await executeQuery()
      
      if (success && isMountedRef.current) {
        // Only start polling if query succeeded
        setPollingActiveState(true)
        
        // Schedule first poll
        pollingTimeoutRef.current = setTimeout(() => {
          if (isMountedRef.current && isPollingActiveRef.current) {
            executePoll()
          }
        }, selectedIntervalRef.current)
        
        // Update refresh_interval in backend
        await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_interval: selectedIntervalRef.current }),
        })
        onUpdate(cell.id, { refresh_interval: selectedIntervalRef.current })
      }
      // If query failed, don't start polling - error will be displayed
    }
  }

  const handleIntervalChange = async (newInterval: number) => {
    setIntervalState(newInterval)
    
    // If currently polling, restart with new interval
    if (isPollingActiveRef.current) {
      // Clear current timeout
      if (pollingTimeoutRef.current) {
        clearTimeout(pollingTimeoutRef.current)
        pollingTimeoutRef.current = null
      }
      
      // Cancel any in-flight requests
      if (abortControllerRef.current) {
        abortControllerRef.current.abort()
        abortControllerRef.current = null
      }
      
      // Restart polling with new interval
      executePoll()
      
      // Update in backend
      if (isMountedRef.current) {
        await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_interval: newInterval }),
        })
        onUpdate(cell.id, { refresh_interval: newInterval })
      }
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
    // First check if we have a saved visualization_mode in config
    if (cell.config?.visualization_mode) {
      return cell.config.visualization_mode as VisualizationMode
    }
    
    // Fallback to legacy visualization_type mapping
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
  
  // Context and namespace state
  const [context, setContext] = useState<string>(cell.config?.context || '')
  const [namespace, setNamespace] = useState<string>(cell.config?.namespace || 'default')
  
  // Logs state
  const [logs, setLogs] = useState<string[]>([])
  const [logsStreaming, setLogsStreaming] = useState(cell.config?.logs_streaming || false)
  const [logFilter, setLogFilter] = useState('')
  const [showTimestamps, setShowTimestamps] = useState(false)
  const [queriedPods, setQueriedPods] = useState<string[]>([])
  const [podColors, setPodColors] = useState<Record<string, string>>({})
  const logsEventSourceRef = useRef<EventSource | null>(null)
  
  // Update streaming state in backend and local state
  const updateLogsStreamingState = useCallback(async (streaming: boolean) => {
    setLogsStreaming(streaming)
    
    const updatedConfig = {
      ...cell.config,
      logs_streaming: streaming
    }
    
    try {
      await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          config: updatedConfig
        }),
      })
      onUpdate(cell.id, {
        config: updatedConfig
      })
    } catch (error) {
      console.error('Failed to update logs streaming state:', error)
    }
  }, [cell.notebook_id, cell.id, cell.config, onUpdate])
  
  // Logs streaming functions
  const startLogsStreaming = useCallback(() => {
    if (queriedPods.length === 0 || !namespace) {
      return
    }

    // Stop any existing stream
    if (logsEventSourceRef.current) {
      logsEventSourceRef.current.close()
    }

    updateLogsStreamingState(true)
    setLogs([])

    const params = new URLSearchParams({
      namespace,
      follow: 'true',
      tail_lines: '100',
    })
    
    if (queriedPods.length === 1) {
      params.append('pod', queriedPods[0])
    } else {
      params.append('pods', queriedPods.join(','))
    }

    const eventSource = new EventSource(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}/logs?${params}`)
    logsEventSourceRef.current = eventSource

    eventSource.onmessage = (event) => {
      try {
        // Parse the log message which should include pod info
        const logData = JSON.parse(event.data)
        const podName = logData.pod || 'unknown'
        const logLine = logData.message || event.data
        
        // Format the log line with colored pod prefix (always show pod name for consistency)
        const formattedLine = `[${podName}] ${logLine}`
          
        setLogs((prev: string[]) => [formattedLine, ...prev])
      } catch (e) {
        // Fallback for non-JSON messages
        setLogs((prev: string[]) => [event.data, ...prev])
      }
    }

    eventSource.addEventListener('error', (event: any) => {
      if (event.data) {
        console.error('Log stream error:', event.data)
      }
    })

    eventSource.addEventListener('end', () => {
      updateLogsStreamingState(false)
      eventSource.close()
    })

    eventSource.onerror = (error) => {
      console.error('EventSource error:', error)
      updateLogsStreamingState(false)
      eventSource.close()
      logsEventSourceRef.current = null
    }
  }, [queriedPods, namespace, cell.notebook_id, cell.id, setLogs, updateLogsStreamingState])
  
  // Process log line to remove timestamp if needed
  const processLogLine = useCallback((line: string) => {
    if (!showTimestamps) {
      // Check if line starts with [podname] prefix
      const podPrefixMatch = line.match(/^(\[[^\]]+\]\s*)(.*)$/)
      if (podPrefixMatch) {
        // Line has [podname] prefix, remove timestamp from the rest
        const podPrefix = podPrefixMatch[1]
        const rest = podPrefixMatch[2]
        const timestampRegex = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z\s*/
        const restWithoutTimestamp = rest.replace(timestampRegex, '')
        return podPrefix + restWithoutTimestamp
      } else {
        // No pod prefix, remove timestamp from beginning
        const timestampRegex = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z\s*/
        return line.replace(timestampRegex, '')
      }
    }
    return line
  }, [showTimestamps])

  // Filter logs based on search term
  const filteredLogs = useMemo(() => {
    return logs.filter(line => {
      if (!logFilter) return true
      const processedLine = processLogLine(line)
      return processedLine.toLowerCase().includes(logFilter.toLowerCase())
    })
  }, [logs, logFilter, processLogLine])
  
  // Check if query contains mutation keywords or in logs mode - memoized to prevent render-time computation
  const isPollingDisabled = useMemo(() => {
    if (!cell.query) return false
    const query = cell.query.toLowerCase()
    const isMutatingQuery = query.includes('create') || query.includes('set') || query.includes('delete')
    const isInLogsMode = currentMode === 'logs'
    return isMutatingQuery || isInLogsMode
  }, [cell.query, currentMode])

  // Stop polling if query becomes a mutation query or in logs mode
  useEffect(() => {
    if (isPollingDisabled && isPollingActiveRef.current) {
      setPollingActiveState(false)
      
      // Clear polling timeout
      if (pollingTimeoutRef.current) {
        clearTimeout(pollingTimeoutRef.current)
        pollingTimeoutRef.current = null
      }
      
      // Cancel any in-flight requests
      if (abortControllerRef.current) {
        abortControllerRef.current.abort()
        abortControllerRef.current = null
      }
    }
  }, [isPollingDisabled])

  // Sync state with cell data when cell changes (fixes persistence on reload)
  useEffect(() => {
    const newMode = getVisualizationMode()
    const newDocMode = getDocumentMode()
    const newGraphMode = getGraphMode()
    
    setCurrentMode(newMode)
    setDocumentMode(newDocMode) 
    setGraphMode(newGraphMode)
  }, [cell.visualization_type, cell.config])

  // Sync polling interval state with cell data (only when actually polling)
  useEffect(() => {
    // Only sync if the cell actually has a positive refresh interval
    // Don't reset to 5000 when polling is paused (refresh_interval = 0)
    if (cell.refresh_interval && cell.refresh_interval > 0) {
      setIntervalState(cell.refresh_interval)
    }
  }, [cell.refresh_interval])

  // Sync cell name state with cell data
  useEffect(() => {
    setCellName(cell.name || '')
  }, [cell.name])

  // Extract pods from results when in logs mode
  useEffect(() => {
    if (currentMode === 'logs' && localResults) {
      const pods = extractPodsFromQueryResult(localResults)
      setQueriedPods(pods)
      
      // Generate colors for each pod
      const colors: Record<string, string> = {}
      pods.forEach(pod => {
        colors[pod] = generatePodColor(pod)
      })
      setPodColors(colors)
    }
  }, [currentMode, localResults])

  // Keep context and namespace in sync with cell config
  useEffect(() => {
    setContext(cell.config?.context || '')
    setNamespace(cell.config?.namespace || 'default')
  }, [cell.config?.context, cell.config?.namespace])

  // Update current mode when cell config changes
  useEffect(() => {
    // Inline the logic to avoid dependency issues
    let newMode: VisualizationMode = 'document'
    
    if (cell.config?.visualization_mode) {
      newMode = cell.config.visualization_mode as VisualizationMode
    } else {
      const vizType = cell.visualization_type
      if (vizType === 'json' || vizType === 'yaml') newMode = 'document'
      else if (vizType === 'table') newMode = 'table'
      else if (vizType === 'graph') newMode = 'graph'
      else newMode = 'document'
    }
    
    setCurrentMode(newMode)
  }, [cell.config?.visualization_mode, cell.visualization_type])

  // Auto-start streaming when switching to logs mode
  const hasAutoStartedRef = useRef(false)
  const isManuallyRestartingRef = useRef(false)
  useEffect(() => {
    if (currentMode === 'logs' && !logsStreaming && queriedPods.length > 0 && namespace && !hasAutoStartedRef.current && !isManuallyRestartingRef.current) {
      hasAutoStartedRef.current = true
      // Start streaming automatically when switching to logs view
      startLogsStreaming()
    }
    // Reset the flag when leaving logs mode
    if (currentMode !== 'logs') {
      hasAutoStartedRef.current = false
    }
  }, [currentMode, logsStreaming, queriedPods.length, namespace, startLogsStreaming])

  // Start streaming when logsStreaming becomes true and we have pods
  useEffect(() => {
    if (currentMode === 'logs' && logsStreaming && queriedPods.length > 0 && namespace && !logsEventSourceRef.current) {
      startLogsStreaming()
    }
  }, [logsStreaming, queriedPods, namespace, currentMode, startLogsStreaming])

  // Autocompletion functions
  const updateSuggestionsPosition = useCallback(() => {
    if (textareaRef.current) {
      const textBeforeCursor = textareaRef.current.value.substring(0, cursorPosition)
      const lines = textBeforeCursor.split('\n')
      const currentLineNumber = lines.length
      const currentLineText = lines[lines.length - 1]

      const lineHeight = 21 // Adjust this value based on your font size and line height
      const charWidth = 8.4 // Adjust this value based on your font size

      // Position one line above the current cursor position to look like it completes the current word
      const top = ((currentLineNumber - 1) * lineHeight) + 15 // Position above current line
      const left = (currentLineText.length * charWidth) + 14

      setSuggestionsPosition({ top, left })
    }
  }, [cursorPosition])

  useEffect(() => {
    updateSuggestionsPosition()
  }, [cursorPosition, updateSuggestionsPosition])

  // Debounced autocomplete function
  const debouncedFetchSuggestions = useCallback(
    debounce(async (query: string, position: number) => {
      try {
        const response = await fetch(`/api/autocomplete?query=${encodeURIComponent(query)}&position=${position}`)
        if (response.ok) {
          const data = await response.json()
          const uniqueSuggestions = Array.from(new Set(data.suggestions || [])) as string[]
          setSuggestions(uniqueSuggestions)
          setSelectedSuggestionIndex(-1)
        }
      } catch (error) {
        console.error('Failed to fetch suggestions:', error)
      }
    }, 300),
    []
  )

  useEffect(() => {
    if (isEditing && query.length > 0) {
      debouncedFetchSuggestions(query, cursorPosition)
    } else {
      setSuggestions([])
    }
  }, [query, cursorPosition, debouncedFetchSuggestions, isEditing])

  const insertSuggestion = useCallback((suggestion: string) => {
    const newQuery = query.slice(0, cursorPosition) + suggestion + query.slice(cursorPosition)
    const newCursorPosition = cursorPosition + suggestion.length

    setQuery(newQuery)
    setCursorPosition(newCursorPosition)
    setSuggestions([])
    setSelectedSuggestionIndex(-1)

    // Update the cursor position in the textarea
    if (textareaRef.current) {
      textareaRef.current.setSelectionRange(newCursorPosition, newCursorPosition)
    }
  }, [query, cursorPosition])

  const scrollSuggestionIntoView = useCallback((index: number) => {
    if (suggestionsRef.current) {
      const suggestionItems = suggestionsRef.current.getElementsByClassName('suggestion-item')
      if (suggestionItems[index]) {
        suggestionItems[index].scrollIntoView({
          behavior: 'smooth',
          block: 'nearest',
        })
      }
    }
  }, [])

  const handleQueryChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const newQuery = e.target.value
    const newPosition = e.target.selectionStart
    setQuery(newQuery)
    setCursorPosition(newPosition)
  }, [])

  const handleCursorChange = useCallback((e: React.SyntheticEvent<HTMLTextAreaElement>) => {
    const newPosition = e.currentTarget.selectionStart
    setCursorPosition(newPosition)
  }, [])

  const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (suggestions.length > 0) {
      if (e.key === 'Tab') {
        e.preventDefault()
        insertSuggestion(suggestions[selectedSuggestionIndex !== -1 ? selectedSuggestionIndex : 0])
      } else if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSelectedSuggestionIndex((prevIndex) =>
          prevIndex < suggestions.length - 1 ? prevIndex + 1 : prevIndex
        )
        scrollSuggestionIntoView(selectedSuggestionIndex + 1)
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedSuggestionIndex((prevIndex) => (prevIndex > 0 ? prevIndex - 1 : 0))
        scrollSuggestionIntoView(selectedSuggestionIndex - 1)
      } else if (e.key === 'Enter' && selectedSuggestionIndex !== -1) {
        e.preventDefault()
        insertSuggestion(suggestions[selectedSuggestionIndex])
      }
    }
  }, [suggestions, selectedSuggestionIndex, insertSuggestion, scrollSuggestionIntoView])

  const handleSuggestionClick = useCallback((e: React.MouseEvent, suggestion: string) => {
    e.preventDefault()
    e.stopPropagation()
    insertSuggestion(suggestion)
    if (textareaRef.current) {
      textareaRef.current.focus()
    }
  }, [insertSuggestion])

  const isEndOfLine = useCallback(() => {
    if (textareaRef.current) {
      const lines = query.split('\n')
      const currentLineIndex = query.substr(0, cursorPosition).split('\n').length - 1
      const currentLine = lines[currentLineIndex]
      return cursorPosition === query.length || 
             (currentLineIndex < lines.length - 1 && 
              cursorPosition === query.indexOf('\n', query.indexOf(currentLine)) - 1)
    }
    return false
  }, [query, cursorPosition])

  const handleModeChange = async (mode: VisualizationMode) => {
    setCurrentMode(mode)
    
    // Map back to legacy visualization_type for backend compatibility
    let vizType: VisualizationType
    if (mode === 'document') {
      vizType = documentMode
    } else if (mode === 'table') {
      vizType = 'table'
    } else if (mode === 'graph') {
      vizType = 'graph'
    } else if (mode === 'logs') {
      // For logs mode, we'll use 'json' as a placeholder since backend expects legacy type
      vizType = 'json'
    } else {
      vizType = 'json'
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
    if (!localLastExecuted) return ''
    if (localError) return 'executed-error'
    return 'executed-success'
  }


  return (
    <div 
      className={`cell ${isEditing ? 'editing' : getExecutionStatus()} ${isPollingActive ? 'polling' : ''} ${isDragging ? 'dragging' : ''} ${isDragOver ? 'drag-over' : ''}`}
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
            onContextChange={(contextValue, namespaceValue) => {
              setContext(contextValue)
              setNamespace(namespaceValue)
              onUpdate(cell.id, {
                config: {
                  ...cell.config,
                  context: contextValue,
                  namespace: namespaceValue
                }
              })
            }}
            className="compact"
          />
          {localLastExecuted && (
            <span className="cell-executed">
              Last executed: {new Date(localLastExecuted).toLocaleTimeString()}
            </span>
          )}
          {isPollingActive && (
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
            <button
              className={`mode-icon ${currentMode === 'logs' ? 'active' : ''}`}
              onClick={() => handleModeChange('logs')}
              title="Logs View"
            >
              <ScrollText size={16} />
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
                  onClick={localError ? () => executeQuery() : handleRunPause} 
                  disabled={localIsRunning && !isPollingActive}
                  className="cell-action execute run-button-main"
                >
                  {currentMode === 'logs' ? (
                    logsStreaming ? (
                      <>
                        <Pause size={14} />
                        Pause
                      </>
                    ) : (
                      <>
                        <Play size={14} />
                        Run
                      </>
                    )
                  ) : isPollingActive && !isPollingDisabled ? (
                    <>
                      <Pause size={14} />
                      Pause
                    </>
                  ) : (
                    <>
                      <Play size={14} />
                      {localIsRunning ? 'Running...' : 'Run'}
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
                ref={textareaRef}
                value={query}
                onChange={handleQueryChange}
                onKeyDown={handleKeyDown}
                onSelect={handleCursorChange}
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
            {isEndOfLine() && suggestions.length > 0 && suggestions[0] !== "" && (
              <div 
                ref={suggestionsRef}
                className="suggestions" 
                style={{ 
                  position: 'absolute',
                  top: `${suggestionsPosition.top}px`, 
                  left: `${suggestionsPosition.left}px`,
                  zIndex: 1000
                }}
              >
                {suggestions.map((suggestion, index) => (
                  <div
                    key={index}
                    className={`suggestion-item ${index === selectedSuggestionIndex ? 'highlighted' : ''}`}
                    onClick={(e) => handleSuggestionClick(e, suggestion)}
                    onMouseDown={(e) => e.preventDefault()} // Prevent blur on click
                  >
                    {suggestion}
                  </div>
                ))}
              </div>
            )}
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

      {localError && (
        <div className="cell-error">
          <strong>Error:</strong> {localError}
        </div>
      )}

      {localResults && (
        <div className="cell-results">
          <div className="cell-results-header">
            <span>
              {currentMode === 'logs' ? `${queriedPods.length} pods found` : 'Results'}
            </span>
            <div className="mode-options">
              {currentMode === 'logs' && (
                <div className="logs-options" style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                  <div className="logs-filter" style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                    <Filter size={12} />
                    <input
                      type="text"
                      placeholder="Filter..."
                      value={logFilter}
                      onChange={(e) => setLogFilter(e.target.value)}
                      style={{
                        padding: '4px 8px',
                        fontSize: '12px',
                        border: '1px solid #e0e4e7',
                        borderRadius: '4px',
                        width: '150px'
                      }}
                    />
                  </div>
                  
                  <button
                    type="button"
                    onClick={() => setShowTimestamps(!showTimestamps)}
                    className={`option-btn ${showTimestamps ? 'active' : ''}`}
                    title={showTimestamps ? 'Hide timestamps' : 'Show timestamps'}
                  >
                    <Clock size={12} />
                  </button>
                </div>
              )}
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
                    className={`option-btn ${graphMode === 'bar' ? 'active' : ''}`}
                    onClick={() => handleGraphModeChange('bar')}
                  >
                    Bar
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
                    code={JSON.stringify(localResults?.data || localResults, null, 2)}
                    language="json"
                  />
                )}
                {documentMode === 'yaml' && (
                  <SyntaxHighlighter
                    code={jsYaml.dump(localResults?.data || localResults)}
                    language="yaml"
                  />
                )}
              </>
            )}
            {currentMode === 'table' && (
              <div className="table-output">
                {renderTableView(localResults?.data || localResults, localResults?.graph, cell.query, cell.id)}
              </div>
            )}
            {currentMode === 'graph' && (
              <div className="graph-output">
                {graphMode === 'force' && renderGraphView(localResults, cell.id, localLastExecuted)}
                {graphMode === 'pie' && renderPieChartView(localResults, cell.query)}
                {graphMode === 'bar' && renderBarChartView(localResults, cell.query)}
              </div>
            )}
            {currentMode === 'logs' && (
              <div className="logs-output">
                {filteredLogs.length > 0 ? (
                  <div className="logs-content" style={{ 
                    fontFamily: 'monospace', 
                    fontSize: '12px', 
                    lineHeight: '1.5',
                    maxHeight: '400px',
                    overflowY: 'auto',
                    backgroundColor: '#f8f9fa',
                    padding: '12px',
                    borderRadius: '4px'
                  }}>
                    {filteredLogs.map((line, index) => {
                      const processedLine = processLogLine(line)
                      
                      // Extract pod name from line if it has the [podname] prefix
                      const podMatch = processedLine.match(/^\[([^\]]+)\]/)
                      const podName = podMatch ? podMatch[1] : null
                      const lineWithoutPrefix = podMatch ? processedLine.replace(/^\[[^\]]+\]\s*/, '') : processedLine
                      
                      return (
                        <div key={index} className="log-line" style={{ marginBottom: '2px' }}>
                          {podName && (
                            <span 
                              className="pod-prefix"
                              style={{ 
                                color: podColors[podName] || '#666',
                                fontWeight: '600',
                                marginRight: '8px'
                              }}
                            >
                              [{podName}]
                            </span>
                          )}
                          <span className="log-content">{podName ? lineWithoutPrefix : processedLine}</span>
                        </div>
                      )
                    })}
                  </div>
                ) : logs.length > 0 ? (
                  <div className="logs-placeholder" style={{ 
                    color: '#666', 
                    fontStyle: 'italic',
                    padding: '24px',
                    textAlign: 'center'
                  }}>
                    No logs match the current filter.
                  </div>
                ) : (
                  <div className="logs-placeholder" style={{ 
                    color: '#666', 
                    fontStyle: 'italic',
                    padding: '24px',
                    textAlign: 'center'
                  }}>
                    {logsStreaming ? 'Waiting for logs...' : queriedPods.length === 0 ? 'No pods found in query results.' : 'Click "Stream" to start streaming logs.'}
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


// Helper functions for logs
const extractPodsFromQueryResult = (result: any): string[] => {
  const pods: string[] = []
  
  // The result might be directly the results object, or wrapped in a results field
  const resultsData = result?.results || result?.result || result
  
  if (!resultsData) {
    return []
  }
  
  // Check graph nodes
  const graph = resultsData.graph || resultsData.Graph
  const nodes = graph?.nodes || graph?.Nodes
  
  if (nodes && Array.isArray(nodes)) {
    for (const node of nodes) {
      // Check both kind and type fields, handle case variations
      const nodeKind = node.kind || node.type || node.Kind || node.Type
      const nodeName = node.name || node.Name
      if (nodeKind === 'Pod' && nodeName) {
        pods.push(nodeName)
      }
    }
  }
  
  // Also check table results in case the query returns pods in table format
  const table = resultsData.table || resultsData.Table
  if (table?.rows && Array.isArray(table.rows)) {
    for (const row of table.rows) {
      // Check if any column contains a pod
      for (const [key, value] of Object.entries(row)) {
        if (typeof value === 'object' && value !== null) {
          const obj = value as any
          if (obj.kind === 'Pod' && obj.metadata?.name) {
            pods.push(obj.metadata.name)
          }
        }
      }
    }
  }
  
  // Try data field as well
  const data = resultsData.data || resultsData.Data
  if (data && Array.isArray(data)) {
    for (const item of data) {
      if (item?.kind === 'Pod' && item?.metadata?.name) {
        pods.push(item.metadata.name)
      }
    }
  }
  
  return [...new Set(pods)] // Remove duplicates
}

const generatePodColor = (podName: string): string => {
  // Generate a consistent color for each pod based on its name
  const colors = [
    '#3498db', '#e74c3c', '#2ecc71', '#f39c12', '#9b59b6',
    '#1abc9c', '#e67e22', '#34495e', '#16a085', '#27ae60',
    '#2980b9', '#8e44ad', '#c0392b', '#d35400', '#7f8c8d'
  ]
  
  let hash = 0
  for (let i = 0; i < podName.length; i++) {
    hash = ((hash << 5) - hash + podName.charCodeAt(i)) & 0xffffffff
  }
  
  return colors[Math.abs(hash) % colors.length]
}