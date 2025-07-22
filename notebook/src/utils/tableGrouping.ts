// Utility functions for table mode grouping logic

export interface ResourceInfo {
  variable: string
  resource: any
}

export const buildK8sResourceMap = (resourcesByVariable: Map<string, any[]>, nodes: any[]): Map<string, ResourceInfo> => {
  const k8sResourceMap = new Map<string, ResourceInfo>()
  
  // Build a map from variable to kind using graph nodes
  // This handles cases where multiple resource types have the same name (like Deployment/api-service and Service/api-service)
  const variableToKind = new Map<string, string>()
  nodes.forEach((node: any) => {
    if (node.Id && node.Kind) {
      variableToKind.set(node.Id, node.Kind)
    }
  })
  
  resourcesByVariable.forEach((resources, variable) => {
    resources.forEach((resource) => {
      const name = resource.name || resource.metadata?.name || ''
      const kind = variableToKind.get(variable) || 'Unknown'
      const k8sId = `${kind}/${name}`
      
      k8sResourceMap.set(k8sId, {variable, resource})
    })
  })
  
  return k8sResourceMap
}

export const findGraphPatterns = (k8sResourceMap: Map<string, ResourceInfo>, edges: any[]): Array<Map<string, any>> => {
  // Build adjacency lists
  const forwardEdges = new Map<string, string[]>()
  const backwardEdges = new Map<string, string[]>()
  
  edges.forEach((edge: any) => {
    const from = edge.From || edge.source
    const to = edge.To || edge.target
    
    if (!forwardEdges.has(from)) {
      forwardEdges.set(from, [])
    }
    forwardEdges.get(from)!.push(to)
    
    if (!backwardEdges.has(to)) {
      backwardEdges.set(to, [])
    }
    backwardEdges.get(to)!.push(from)
  })
  
  // Find root nodes (nodes with no incoming edges)
  const rootNodes = new Set<string>()
  k8sResourceMap.forEach((_, k8sId) => {
    if (!backwardEdges.has(k8sId)) {
      rootNodes.add(k8sId)
    }
  })
  
  // Find all connected components (pattern matches) using proper visited tracking
  const patternMatches: Array<Set<string>> = []
  const visited = new Set<string>()
  
  const findConnectedComponent = (startId: string): Set<string> => {
    const component = new Set<string>()
    const queue = [startId]
    
    while (queue.length > 0) {
      const currentId = queue.shift()!
      if (component.has(currentId) || visited.has(currentId)) continue
      
      component.add(currentId)
      visited.add(currentId)  // Mark as visited globally
      
      // Add all connected nodes (both forward and backward)
      const successors = forwardEdges.get(currentId) || []
      const predecessors = backwardEdges.get(currentId) || []
      
      successors.forEach(connectedId => {
        if (!component.has(connectedId) && !visited.has(connectedId)) {
          queue.push(connectedId)
        }
      })
      
      predecessors.forEach(connectedId => {
        if (!component.has(connectedId) && !visited.has(connectedId)) {
          queue.push(connectedId)
        }
      })
    }
    
    return component
  }
  
  // Find all connected components from all nodes (not just root nodes)
  k8sResourceMap.forEach((_, k8sId) => {
    if (!visited.has(k8sId)) {
      const component = findConnectedComponent(k8sId)
      if (component.size > 0) {
        patternMatches.push(component)
      }
    }
  })
  
  // Convert each connected component to individual resource patterns
  // Following Go implementation: each resource becomes a separate row with same pattern ID
  const patterns: Array<Map<string, any>> = []
  
  patternMatches.forEach((component, patternId) => {
    component.forEach(k8sId => {
      const resourceInfo = k8sResourceMap.get(k8sId)
      if (resourceInfo) {
        const pattern = new Map<string, any>()
        pattern.set(resourceInfo.variable, resourceInfo.resource)
        pattern.set('_patternId', patternId) // Add pattern ID for grouping
        patterns.push(pattern)
      }
    })
  })
  
  return patterns
}

export const extractFieldValue = (resource: any, fieldPath: string, resourcesByVariable: Map<string, any[]>): any => {
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

export const buildTableRows = (
  patterns: Array<Map<string, any>>, 
  columns: string[], 
  resourcesByVariable: Map<string, any[]>
): Array<Record<string, any>> => {
  // Group patterns by pattern ID to recreate connected components
  const patternGroups = new Map<number, Array<Map<string, any>>>()
  
  patterns.forEach(pattern => {
    const patternId = pattern.get('_patternId') as number
    if (!patternGroups.has(patternId)) {
      patternGroups.set(patternId, [])
    }
    patternGroups.get(patternId)!.push(pattern)
  })
  
  const rows: Array<Record<string, any>> = []
  
  // For each pattern group (connected component), create rows
  patternGroups.forEach((patternResources, patternId) => {
    // Build a map of variables to their resources for this pattern
    const resourcesByVar = new Map<string, any[]>()
    patternResources.forEach(pattern => {
      pattern.forEach((value, key) => {
        if (key !== '_patternId') {
          if (!resourcesByVar.has(key)) {
            resourcesByVar.set(key, [])
          }
          resourcesByVar.get(key)!.push(value)
        }
      })
    })
    
    // Find the variable with the most resources - this determines how many rows we create
    let maxCount = 0
    let primaryVariable = ''
    resourcesByVar.forEach((resources, variable) => {
      if (resources.length > maxCount) {
        maxCount = resources.length
        primaryVariable = variable
      }
    })
    
    // Create one table row for each instance of the primary variable
    // Each row will contain data from all variables in this pattern
    const primaryResources = resourcesByVar.get(primaryVariable) || []
    
    primaryResources.forEach((primaryResource, index) => {
      const row: Record<string, any> = {}
      
      // For each column, get the value from the appropriate resource
      columns.forEach(field => {
        const [variable] = field.split('.')
        
        let resource
        if (variable === primaryVariable) {
          // Use the specific instance for the primary variable (e.g., specific pod)
          resource = primaryResource
        } else {
          // Use the first (and typically only) instance for other variables (e.g., deployment, service)
          const variableResources = resourcesByVar.get(variable) || []
          resource = variableResources[0] // There should only be one deployment, one service per pattern
        }
        
        if (resource) {
          const value = extractFieldValue(resource, field, resourcesByVariable)
          row[field] = value
        } else {
          row[field] = null
        }
      })
      
      // Only add the row if it has at least one non-null value
      const hasData = Object.values(row).some(value => value !== null)
      if (hasData) {
        rows.push(row)
      }
    })
  })
  
  return rows
}