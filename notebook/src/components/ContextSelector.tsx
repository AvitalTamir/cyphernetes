import React, { useState, useRef, useEffect, useCallback, useMemo } from 'react'
import { ChevronDown } from 'lucide-react'
import './ContextSelector.css'

interface ContextInfo {
  context: string
  namespace: string
}

interface ContextSelectorProps {
  context?: string
  namespace?: string
  onContextChange: (context: string, namespace: string) => void
  className?: string
  disabled?: boolean
}

export const ContextSelector: React.FC<ContextSelectorProps> = ({
  context,
  namespace,
  onContextChange,
  className = '',
  disabled = false
}) => {
  const [contextInfo, setContextInfo] = useState<ContextInfo | null>(null)
  const [namespaces, setNamespaces] = useState<string[]>([])
  const [isNamespaceSelectorOpen, setIsNamespaceSelectorOpen] = useState(false)
  const [namespaceFilter, setNamespaceFilter] = useState('')
  
  const namespaceSelectorRef = useRef<HTMLDivElement>(null)
  const namespaceSearchRef = useRef<HTMLInputElement>(null)
  const namespaceElementRef = useRef<HTMLSpanElement>(null)
  const [namespaceSelectorPosition, setNamespaceSelectorPosition] = useState({ top: 0, left: 0 })

  // Fetch current context info
  const fetchContextInfo = useCallback(async () => {
    try {
      const response = await fetch('/api/context')
      if (!response.ok) {
        throw new Error('Failed to fetch context info')
      }
      const data = await response.json()
      setContextInfo(data)
      
      // If this cell doesn't have context/namespace set, use the current ones
      if (!context || !namespace) {
        onContextChange(context || data.context, namespace || data.namespace)
      }
    } catch (error) {
      console.error('Failed to fetch context info:', error)
    }
  }, [context, namespace, onContextChange])

  // Fetch available namespaces
  const fetchNamespaces = useCallback(async () => {
    try {
      const response = await fetch('/api/namespaces')
      if (!response.ok) {
        throw new Error('Failed to fetch namespaces')
      }
      const data = await response.json()
      setNamespaces(data.namespaces || [])
    } catch (error) {
      console.error('Failed to fetch namespaces:', error)
    }
  }, [])

  useEffect(() => {
    fetchContextInfo()
    fetchNamespaces()
  }, [fetchContextInfo, fetchNamespaces])

  const handleNamespaceClick = (e: React.MouseEvent) => {
    if (disabled) return
    
    e.preventDefault()
    e.stopPropagation()
    
    // Calculate position for the namespace selector
    if (namespaceElementRef.current) {
      const rect = namespaceElementRef.current.getBoundingClientRect()
      setNamespaceSelectorPosition({
        top: rect.bottom + 5,
        left: rect.left,
      })
    }
    
    setIsNamespaceSelectorOpen(!isNamespaceSelectorOpen)
    
    // Focus the search input when opening
    if (!isNamespaceSelectorOpen) {
      setTimeout(() => {
        if (namespaceSearchRef.current) {
          namespaceSearchRef.current.focus()
        }
      }, 100)
    }
  }

  const handleNamespaceSelect = (selectedNamespace: string) => {
    // Update this cell's context/namespace
    onContextChange(context || contextInfo?.context || '', selectedNamespace)
    setIsNamespaceSelectorOpen(false)
    setNamespaceFilter('')
  }

  // Close namespace selector when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        namespaceSelectorRef.current && 
        !namespaceSelectorRef.current.contains(event.target as Node)
      ) {
        setIsNamespaceSelectorOpen(false)
      }
    }

    if (isNamespaceSelectorOpen) {
      document.addEventListener('mousedown', handleClickOutside)
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [isNamespaceSelectorOpen])

  // Filter namespaces based on search input
  const filteredNamespaces = useMemo(() => {
    if (!namespaceFilter) return namespaces
    return namespaces.filter(ns => 
      ns.toLowerCase().includes(namespaceFilter.toLowerCase())
    )
  }, [namespaces, namespaceFilter])

  const displayContext = context || contextInfo?.context || 'default'
  const displayNamespace = namespace || contextInfo?.namespace || 'default'

  return (
    <div className={`context-selector ${className}`}>
      <div className="context-info">
        <span className="context-label">ctx:</span>
        <span className="context-value">{displayContext}</span>
        
        <span className="namespace-label">ns:</span>
        <span 
          ref={namespaceElementRef}
          className={`namespace-value ${disabled ? 'disabled' : ''}`}
          onClick={handleNamespaceClick}
          title={disabled ? "Namespace selection disabled in shared mode" : "Click to change namespace"}
          style={{ cursor: disabled ? 'not-allowed' : 'pointer' }}
        >
          {displayNamespace}
          <ChevronDown size={12} className="namespace-chevron" />
        </span>
        
        {isNamespaceSelectorOpen && (
          <div 
            className="namespace-selector" 
            ref={namespaceSelectorRef}
            style={{
              position: 'fixed',
              top: `${namespaceSelectorPosition.top}px`,
              left: `${namespaceSelectorPosition.left}px`,
              maxHeight: '300px',
              width: '250px',
              overflowY: 'auto',
              zIndex: 9999,
            }}
          >
            <div className="namespace-search">
              <input
                ref={namespaceSearchRef}
                type="text"
                placeholder="Search namespaces..."
                value={namespaceFilter}
                onChange={(e) => setNamespaceFilter(e.target.value)}
              />
            </div>
            <div className="namespace-list">
              {filteredNamespaces.length > 0 ? (
                filteredNamespaces.map(ns => (
                  <div 
                    key={ns} 
                    className={`namespace-item ${displayNamespace === ns ? 'active' : ''}`}
                    onClick={() => handleNamespaceSelect(ns)}
                  >
                    {ns}
                  </div>
                ))
              ) : (
                <div className="namespace-item disabled">
                  {namespaces.length === 0 ? 'Loading namespaces...' : 'No namespaces found'}
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}