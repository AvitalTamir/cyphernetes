import { describe, it, expect } from 'vitest'
import { buildK8sResourceMap, findGraphPatterns, extractFieldValue, buildTableRows } from '../../utils/tableGrouping'

// Mock data structures that match the EXACT production format from Cyphernetes
const createProductionLikeData = () => {
  // Resources matching production format - note name collisions between deployments and services
  const deployment1 = {
    name: 'api-service',  // Same name as service!
    spec: { replicas: 2 }
  }
  
  const deployment2 = {
    name: 'billing-service',  // Same name as service!
    spec: { replicas: 3 }
  }
  
  const deployment3 = {
    name: 'users-service',  // Same name as service!
    spec: { replicas: 1 }
  }
  
  const service1 = {
    name: 'api-service',  // Same name as deployment!
    spec: { clusterIP: '10.96.138.152' }
  }
  
  const service2 = {
    name: 'billing-service',  // Same name as deployment!
    spec: { clusterIP: '10.96.85.12' }
  }
  
  const service3 = {
    name: 'users-service',  // Same name as deployment!
    spec: { clusterIP: '10.96.63.208' }
  }
  
  const pod1 = {
    name: 'api-service-78746f6cc4-mvkbb',
    status: { phase: 'Running' }
  }
  
  const pod2 = {
    name: 'api-service-78746f6cc4-r4m2g',
    status: { phase: 'Running' }
  }
  
  const pod3 = {
    name: 'billing-service-74c7dcb885-4kwkq',
    status: { phase: 'Running' }
  }
  
  const pod4 = {
    name: 'billing-service-74c7dcb885-cq7xw',
    status: { phase: 'Running' }
  }
  
  const pod5 = {
    name: 'billing-service-74c7dcb885-zbqqp',
    status: { phase: 'Running' }
  }
  
  const pod6 = {
    name: 'users-service-56bb47d84c-b8ldj',
    status: { phase: 'Running' }
  }
  
  return {
    resources: {
      d: [deployment1, deployment2, deployment3],
      s: [service1, service2, service3], 
      p: [pod1, pod2, pod3, pod4, pod5, pod6]
    },
    graph: {
      // EXACT production node structure with duplicates and name collisions
      Nodes: [
        { Id: 's', Kind: 'Service', Name: 'api-service', Namespace: 'default' },
        { Id: 's', Kind: 'Service', Name: 'billing-service', Namespace: 'default' },
        { Id: 's', Kind: 'Service', Name: 'users-service', Namespace: 'default' },
        { Id: 'd', Kind: 'Deployment', Name: 'api-service', Namespace: 'default' },
        { Id: 'd', Kind: 'Deployment', Name: 'billing-service', Namespace: 'default' },
        { Id: 'd', Kind: 'Deployment', Name: 'users-service', Namespace: 'default' },
        { Id: 'p', Kind: 'Pod', Name: 'api-service-78746f6cc4-mvkbb', Namespace: 'default' },
        { Id: 'p', Kind: 'Pod', Name: 'api-service-78746f6cc4-r4m2g', Namespace: 'default' },
        { Id: 'p', Kind: 'Pod', Name: 'billing-service-74c7dcb885-4kwkq', Namespace: 'default' },
        { Id: 'p', Kind: 'Pod', Name: 'billing-service-74c7dcb885-cq7xw', Namespace: 'default' },
        { Id: 'p', Kind: 'Pod', Name: 'billing-service-74c7dcb885-zbqqp', Namespace: 'default' },
        { Id: 'p', Kind: 'Pod', Name: 'users-service-56bb47d84c-b8ldj', Namespace: 'default' },
        // Some duplicates like in production 
        { Id: 's', Kind: 'Service', Name: 'api-service', Namespace: 'default' },
        { Id: 'd', Kind: 'Deployment', Name: 'api-service', Namespace: 'default' }
      ],
      // EXACT production edge structure and directions
      Edges: [
        { From: 'Service/api-service', To: 'Deployment/api-service', Type: 'SERVICE_EXPOSE_DEPLOYMENT' },
        { From: 'Service/billing-service', To: 'Deployment/billing-service', Type: 'SERVICE_EXPOSE_DEPLOYMENT' },
        { From: 'Service/users-service', To: 'Deployment/users-service', Type: 'SERVICE_EXPOSE_DEPLOYMENT' },
        { From: 'Pod/api-service-78746f6cc4-mvkbb', To: 'Service/api-service', Type: 'SERVICE_EXPOSE_POD' },
        { From: 'Pod/api-service-78746f6cc4-r4m2g', To: 'Service/api-service', Type: 'SERVICE_EXPOSE_POD' },
        { From: 'Pod/billing-service-74c7dcb885-4kwkq', To: 'Service/billing-service', Type: 'SERVICE_EXPOSE_POD' },
        { From: 'Pod/billing-service-74c7dcb885-cq7xw', To: 'Service/billing-service', Type: 'SERVICE_EXPOSE_POD' },
        { From: 'Pod/billing-service-74c7dcb885-zbqqp', To: 'Service/billing-service', Type: 'SERVICE_EXPOSE_POD' },
        { From: 'Pod/users-service-56bb47d84c-b8ldj', To: 'Service/users-service', Type: 'SERVICE_EXPOSE_POD' }
      ]
    },
    returnFields: ['d.name', 'd.spec.replicas', 'p.name', 'p.status.phase', 's.name', 's.spec.clusterIP']
  }
}

describe('QueryCell Table Mode Grouping - Production Data', () => {
  it('should correctly group resources with name collisions and production data structure', () => {
    const mockData = createProductionLikeData()
    const resourcesByVariable = new Map(Object.entries(mockData.resources))
    
    // Use the same utility functions as the main component
    const nodes = mockData.graph.Nodes || []
    const edges = mockData.graph.Edges || []
    const k8sResourceMap = buildK8sResourceMap(resourcesByVariable, nodes)
    const patterns = findGraphPatterns(k8sResourceMap, edges)
    const rows = buildTableRows(patterns, mockData.returnFields, resourcesByVariable)
    
    console.log('Production test - connected components:', new Set(patterns.map(p => p.get('_patternId'))).size)
    console.log('Production test - total patterns:', patterns.length)
    console.log('Production test - total rows:', rows.length)
    console.log('Production test - sample rows:', rows.slice(0, 3))
    
    // Should have exactly 3 connected components (api-service, billing-service, users-service)
    const uniquePatternIds = new Set(patterns.map(p => p.get('_patternId')))
    expect(uniquePatternIds.size).toBe(3)
    
    // Should have 6 table rows total (2 api pods + 3 billing pods + 1 users pod)
    expect(rows.length).toBe(6)
    
    // Each row should have deployment, service, and pod data combined
    rows.forEach(row => {
      expect(row['d.name']).toBeDefined() // Deployment name
      expect(row['d.spec.replicas']).toBeDefined() // Deployment replicas  
      expect(row['s.name']).toBeDefined() // Service name
      expect(row['s.spec.clusterIP']).toBeDefined() // Service clusterIP
      expect(row['p.name']).toBeDefined() // Pod name
      expect(row['p.status.phase']).toBe('Running') // Pod phase
      
      // Deployment and service should have same name (this was the production issue)
      expect(row['d.name']).toBe(row['s.name'])
    })
    
    // Test api-service group
    const apiRows = rows.filter(row => row['d.name'] === 'api-service')
    expect(apiRows.length).toBe(2) // 2 api pods
    apiRows.forEach(row => {
      expect(row['d.spec.replicas']).toBe(2)
      expect(row['s.spec.clusterIP']).toBe('10.96.138.152')
      expect(row['p.name']).toMatch(/^api-service-/)
    })
    
    // Test billing-service group  
    const billingRows = rows.filter(row => row['d.name'] === 'billing-service')
    expect(billingRows.length).toBe(3) // 3 billing pods
    billingRows.forEach(row => {
      expect(row['d.spec.replicas']).toBe(3)
      expect(row['s.spec.clusterIP']).toBe('10.96.85.12')
      expect(row['p.name']).toMatch(/^billing-service-/)
    })
    
    // Test users-service group
    const usersRows = rows.filter(row => row['d.name'] === 'users-service')
    expect(usersRows.length).toBe(1) // 1 users pod
    usersRows.forEach(row => {
      expect(row['d.spec.replicas']).toBe(1)
      expect(row['s.spec.clusterIP']).toBe('10.96.63.208')
      expect(row['p.name']).toMatch(/^users-service-/)
    })
  })

  it('should handle variable to kind mapping correctly', () => {
    const mockData = createProductionLikeData()
    const resourcesByVariable = new Map(Object.entries(mockData.resources))
    const nodes = mockData.graph.Nodes || []
    
    const k8sResourceMap = buildK8sResourceMap(resourcesByVariable, nodes)
    
    // Check that deployments are mapped correctly despite name collisions
    expect(k8sResourceMap.has('Deployment/api-service')).toBe(true)
    expect(k8sResourceMap.has('Deployment/billing-service')).toBe(true)
    expect(k8sResourceMap.has('Service/api-service')).toBe(true)
    expect(k8sResourceMap.has('Service/billing-service')).toBe(true)
    
    // Verify variable assignments
    expect(k8sResourceMap.get('Deployment/api-service')?.variable).toBe('d')
    expect(k8sResourceMap.get('Service/api-service')?.variable).toBe('s')
    expect(k8sResourceMap.get('Pod/api-service-78746f6cc4-mvkbb')?.variable).toBe('p')
  })
})