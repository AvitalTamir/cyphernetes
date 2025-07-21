import React, { useState, useEffect, useRef, useCallback } from 'react'
import { Cell } from '../types/notebook'
import { ScrollText, Edit3, Trash2, Save, X, Play, Square, ChevronDown, Filter, Clock } from 'lucide-react'
import { ContextSelector } from './ContextSelector'
import './CellComponent.css'

interface LogsCellProps {
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

export const LogsCell: React.FC<LogsCellProps> = ({
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
  const [isEditing, setIsEditing] = useState(!cell.query)
  const [podName, setPodName] = useState(cell.query || '')
  const [isEditingName, setIsEditingName] = useState(false)
  const [cellName, setCellName] = useState(cell.name || '')
  const [logs, setLogs] = useState<string[]>([])
  const [error, setError] = useState<string | null>(null)
  const [isStreaming, setIsStreaming] = useState(false)
  const [streamingSuccess, setStreamingSuccess] = useState(false)
  const [context, setContext] = useState<string>('')
  const [namespace, setNamespace] = useState<string>('default')
  const [availablePods, setAvailablePods] = useState<string[]>([])
  const [isPodSelectorOpen, setIsPodSelectorOpen] = useState(false)
  const [podFilter, setPodFilter] = useState('')
  const [dropdownPosition, setDropdownPosition] = useState({ top: 0, left: 0 })
  const [logFilter, setLogFilter] = useState('')
  const [showTimestamps, setShowTimestamps] = useState(false)
  
  const eventSourceRef = useRef<EventSource | null>(null)
  const logsEndRef = useRef<HTMLDivElement>(null)
  const isMountedRef = useRef(true)
  const podSelectorRef = useRef<HTMLDivElement>(null)
  const podInputRef = useRef<HTMLInputElement>(null)
  const hasAutoStartedRef = useRef(false)
  const manuallyStoppedRef = useRef(false)

  // Sync cell name state with cell data
  useEffect(() => {
    setCellName(cell.name || '')
  }, [cell.name])


  // Clean up on unmount
  useEffect(() => {
    return () => {
      isMountedRef.current = false
      if (eventSourceRef.current) {
        eventSourceRef.current.close()
      }
    }
  }, [])

  // Fetch available pods when namespace changes
  const fetchPods = useCallback(async (ns: string) => {
    if (!ns) return
    
    console.log('Fetching pods for namespace:', ns)
    try {
      const response = await fetch(`/api/pods?namespace=${encodeURIComponent(ns)}`)
      console.log('Pod fetch response status:', response.status)
      if (!response.ok) {
        throw new Error('Failed to fetch pods')
      }
      const data = await response.json()
      console.log('Fetched pods data:', data)
      setAvailablePods(data.pods || [])
    } catch (err) {
      console.error('Failed to fetch pods:', err)
      setAvailablePods([])
    }
  }, [])

  // Auto-start streaming when pod is selected and not in editing mode
  useEffect(() => {
    console.log('Auto-start effect triggered:', {
      podName,
      namespace,
      isEditing,
      isStreaming,
      hasAutoStarted: hasAutoStartedRef.current,
      manuallyStopped: manuallyStoppedRef.current
    })
    
    if (podName && namespace && !isEditing && !isStreaming && !hasAutoStartedRef.current && !manuallyStoppedRef.current) {
      console.log('Auto-starting streaming...')
      hasAutoStartedRef.current = true
      startStreaming()
    }
    
    // Reset the flag when pod changes or when we stop streaming
    if (!podName || !isStreaming) {
      hasAutoStartedRef.current = false
    }
  }, [podName, namespace, isEditing, isStreaming])

  // Fetch pods when namespace changes
  useEffect(() => {
    if (namespace) {
      fetchPods(namespace)
    }
  }, [namespace, fetchPods])

  // Initial fetch when component mounts or editing starts
  useEffect(() => {
    if (isEditing && namespace) {
      fetchPods(namespace)
    }
  }, [isEditing, namespace, fetchPods])

  // Fetch pods on initial mount with default namespace
  useEffect(() => {
    fetchPods('default')
  }, [])

  // Close pod selector when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        podSelectorRef.current && 
        !podSelectorRef.current.contains(event.target as Node)
      ) {
        setIsPodSelectorOpen(false)
      }
    }

    if (isPodSelectorOpen) {
      document.addEventListener('mousedown', handleClickOutside)
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [isPodSelectorOpen])

  const handleSave = async () => {
    try {
      const response = await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ query: podName }),
      })
      
      if (response.ok) {
        onUpdate(cell.id, { query: podName })
        setIsEditing(false)
      }
    } catch (error) {
      console.error('Failed to save logs cell:', error)
    }
  }

  const handleCancel = () => {
    setPodName(cell.query || '')
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

  const startStreaming = () => {
    if (!podName || !namespace) return

    console.log('startStreaming called')
    // Reset manually stopped flag when manually starting
    manuallyStoppedRef.current = false

    // Stop any existing stream
    if (eventSourceRef.current) {
      eventSourceRef.current.close()
    }

    setIsStreaming(true)
    setStreamingSuccess(false)
    setError(null)
    setLogs([])

    const params = new URLSearchParams({
      pod: podName,
      namespace,
      follow: 'true',
      tail_lines: '100',
    })

    const eventSource = new EventSource(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}/logs?${params}`)
    eventSourceRef.current = eventSource

    eventSource.onmessage = (event) => {
      if (isMountedRef.current) {
        setLogs(prev => [event.data, ...prev])
        setStreamingSuccess(true)
      }
    }

    eventSource.addEventListener('error', (event: any) => {
      if (event.data && isMountedRef.current) {
        setError(event.data)
        setStreamingSuccess(false)
      }
    })

    eventSource.addEventListener('end', () => {
      if (isMountedRef.current) {
        setIsStreaming(false)
        eventSource.close()
      }
    })

    eventSource.onerror = () => {
      if (isMountedRef.current) {
        setError('Connection to log stream lost')
        setIsStreaming(false)
        setStreamingSuccess(false)
        eventSource.close()
      }
    }
  }

  const stopStreaming = () => {
    console.log('stopStreaming called, current eventSource:', eventSourceRef.current)
    console.log('current isStreaming state:', isStreaming)
    
    // Set manually stopped flag to prevent auto-restart
    manuallyStoppedRef.current = true
    console.log('Set manuallyStoppedRef to true')
    
    if (eventSourceRef.current) {
      console.log('Closing eventSource...')
      eventSourceRef.current.close()
      eventSourceRef.current = null
      console.log('EventSource closed and ref set to null')
    } else {
      console.log('No eventSource to close')
    }
    
    setIsStreaming(false)
    setStreamingSuccess(false)
    console.log('Set isStreaming to false')
  }

  const handleContextChange = (newContext: string, newNamespace: string) => {
    setContext(newContext)
    setNamespace(newNamespace)
  }

  const handlePodSelect = (selectedPod: string) => {
    setPodName(selectedPod)
    setIsPodSelectorOpen(false)
    setPodFilter('')
    // Save the pod selection immediately
    fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ query: selectedPod }),
    }).then(response => {
      if (response.ok) {
        onUpdate(cell.id, { query: selectedPod })
      }
    }).catch(error => {
      console.error('Failed to save pod selection:', error)
    })
  }

  const filteredPods = availablePods.filter(pod => 
    pod.toLowerCase().includes(podFilter.toLowerCase())
  )

  // Process log line to remove timestamp if needed
  const processLogLine = (line: string) => {
    if (!showTimestamps) {
      // Remove ISO timestamp from the beginning of the line
      const timestampRegex = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z\s*/
      return line.replace(timestampRegex, '')
    }
    return line
  }

  // Filter logs based on search term
  const filteredLogs = logs.filter(line => {
    if (!logFilter) return true
    const processedLine = processLogLine(line)
    return processedLine.toLowerCase().includes(logFilter.toLowerCase())
  })

  const handleDragStart = (e: React.DragEvent) => {
    e.dataTransfer.setData('text/plain', cell.id)
    e.dataTransfer.effectAllowed = 'move'
    
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

  const getExecutionStatus = () => {
    if (error) return 'executed-error'
    if (streamingSuccess && isStreaming) return 'executed-success'
    return ''
  }

  const cellClasses = [
    'cell',
    'logs-cell',
    isEditing ? 'editing' : getExecutionStatus(),
    isStreaming ? 'polling' : '',
    isDragging ? 'dragging' : '',
    isDragOver ? 'drag-over' : ''
  ].filter(Boolean).join(' ')

  return (
    <div 
      className={cellClasses}
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
            <ScrollText size={12} />
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
                {cellName || 'Logs'}
              </span>
            )}
          </span>
          <ContextSelector
            context={context}
            namespace={namespace}
            onContextChange={handleContextChange}
            className="cell-context"
          />
          {isStreaming && (
            <span className="cell-polling-indicator">
              Live
            </span>
          )}
        </div>

        <div className="cell-actions">
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
              {podName && (
                <>
                  {isStreaming ? (
                    <button 
                      type="button"
                      onClick={(e) => {
                        e.preventDefault()
                        stopStreaming()
                      }}
                      className="cell-action"
                      title="Stop streaming logs"
                    >
                      <Square size={14} />
                      Stop
                    </button>
                  ) : (
                    <button 
                      type="button"
                      onClick={(e) => {
                        e.preventDefault()
                        startStreaming()
                      }}
                      className="cell-action execute"
                      title="Start streaming logs"
                    >
                      <Play size={14} />
                      Stream
                    </button>
                  )}
                </>
              )}
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
          <div className="logs-editor">
            <div className="pod-selector-container">
              <div className="pod-selector-label">Select Pod:</div>
              <div className="pod-selector" ref={podSelectorRef}>
                <div className="pod-selector-display">
                  <input
                    ref={podInputRef}
                    type="text"
                    value={podName}
                    onChange={(e) => {
                      setPodName(e.target.value)
                      // Use the input value directly for filtering
                      const inputValue = e.target.value
                      const filtered = availablePods.filter(pod => 
                        pod.toLowerCase().includes(inputValue.toLowerCase())
                      )
                      console.log('Filtering pods with input:', inputValue, 'filtered:', filtered)
                    }}
                    placeholder="Type to search pods or enter pod name..."
                    className="logs-pod-input"
                    onFocus={() => {
                      // Calculate position for dropdown
                      if (podInputRef.current) {
                        const rect = podInputRef.current.getBoundingClientRect()
                        setDropdownPosition({
                          top: rect.bottom + 4,
                          left: rect.left
                        })
                      }
                      setIsPodSelectorOpen(true)
                    }}
                  />
                  <ChevronDown size={14} className="pod-selector-chevron" />
                </div>
                
                {isPodSelectorOpen && (() => {
                  console.log('Pod dropdown is open:', isPodSelectorOpen)
                  // Filter pods based on the main input value (podName)
                  const currentFilteredPods = availablePods.filter(pod => 
                    pod.toLowerCase().includes(podName.toLowerCase())
                  )
                  
                  return (
                    <div 
                      className="pod-dropdown"
                      style={{
                        top: `${dropdownPosition.top}px`,
                        left: `${dropdownPosition.left}px`
                      }}
                    >
                      <div className="pod-list">
                        {currentFilteredPods.length > 0 ? (
                          currentFilteredPods.map(pod => (
                            <div 
                              key={pod} 
                              className={`pod-item ${podName === pod ? 'active' : ''}`}
                              onClick={() => handlePodSelect(pod)}
                            >
                              {pod}
                            </div>
                          ))
                        ) : (
                          <div className="pod-item disabled">
                            {availablePods.length === 0 ? 'Loading pods...' : `No pods found matching "${podName}"`}
                          </div>
                        )}
                      </div>
                    </div>
                  )
                })()}
              </div>
            </div>
            <div className="editor-hint">Select a pod from the dropdown or type a pod name, then save</div>
          </div>
        ) : (
          <>
            {error && (
              <div className="cell-error">
                <strong>Error:</strong> {error}
              </div>
            )}
            
            {podName ? (
              <div className="logs-content">
                <div className="logs-info">
                  <div className="logs-pod-name">
                    <strong>Pod:</strong> {podName}
                  </div>
                  {logs.length > 0 && (
                    <div className="logs-controls">
                      <div className="logs-filter">
                        <Filter size={12} />
                        <input
                          type="text"
                          placeholder="Filter..."
                          value={logFilter}
                          onChange={(e) => setLogFilter(e.target.value)}
                          className="logs-filter-input"
                        />
                      </div>
                      <button
                        type="button"
                        onClick={() => setShowTimestamps(!showTimestamps)}
                        className={`logs-timestamp-toggle ${showTimestamps ? 'active' : ''}`}
                        title={showTimestamps ? 'Hide timestamps' : 'Show timestamps'}
                      >
                        <Clock size={12} />
                      </button>
                    </div>
                  )}
                </div>
                
                {filteredLogs.length > 0 ? (
                  <div className="logs-output">
                    {filteredLogs.map((line, index) => (
                      <div key={index} className="log-line">
                        {processLogLine(line)}
                      </div>
                    ))}
                  </div>
                ) : logs.length > 0 ? (
                  <div className="logs-placeholder">
                    No logs match the current filter.
                  </div>
                ) : (
                  <div className="logs-placeholder">
                    {isStreaming ? 'Waiting for logs...' : 'No logs to display. Click "Stream" to start streaming logs.'}
                  </div>
                )}
              </div>
            ) : (
              <div className="cell-empty">
                <p>No pod configured</p>
                <button onClick={() => setIsEditing(true)} className="btn btn-primary">
                  Select Pod
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}