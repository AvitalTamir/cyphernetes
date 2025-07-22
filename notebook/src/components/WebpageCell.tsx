import React, { useState, useEffect, useRef } from 'react'
import { Cell } from '../types/notebook'
import { Globe, Edit3, Trash2, Save, X, ExternalLink, RefreshCw, AlertCircle } from 'lucide-react'
import './CellComponent.css'

interface WebpageCellProps {
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

export const WebpageCell: React.FC<WebpageCellProps> = ({
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
  const [isEditing, setIsEditing] = useState(!cell.query) // Start editing if empty
  const [url, setUrl] = useState(cell.query || '')
  const [isEditingName, setIsEditingName] = useState(false)
  const [cellName, setCellName] = useState(cell.name || '')
  const [iframeKey, setIframeKey] = useState(0)
  const [loadError, setLoadError] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [loadSuccess, setLoadSuccess] = useState(false)
  const [isChecking, setIsChecking] = useState(false)
  const iframeRef = useRef<HTMLIFrameElement>(null)
  const objectRef = useRef<HTMLObjectElement>(null)

  // Sync cell name state with cell data
  useEffect(() => {
    setCellName(cell.name || '')
  }, [cell.name])

  // Reset state when URL changes
  useEffect(() => {
    setLoadError(false)
    setErrorMessage('')
    setIsLoading(false)
    setLoadSuccess(false)
    setIsChecking(false)
  }, [url, iframeKey])

  // Test iframe blocking using a temporary unsandboxed iframe
  useEffect(() => {
    if (!url || !isValidUrl(url) || isEditing) return

    setIsChecking(true)

    const checkTimer = setTimeout(() => {
      // Create a temporary iframe without sandbox for testing
      const testIframe = document.createElement('iframe')
      testIframe.style.position = 'absolute'
      testIframe.style.top = '-9999px'
      testIframe.style.left = '-9999px'
      testIframe.style.width = '1px'
      testIframe.style.height = '1px'
      testIframe.style.visibility = 'hidden'
      testIframe.src = url
      
      let isResolved = false
      
      // Handle iframe load events
      testIframe.onload = () => {
        if (isResolved) return
        setTimeout(() => {
          if (isResolved) return
          
          try {
            const testDoc = testIframe.contentDocument || testIframe.contentWindow?.document
            
            if (testDoc) {
              // We can access the document - check if it's actually blocked
              const body = testDoc.body
              const hasRealContent = body && (
                body.children.length > 0 || 
                (body.textContent && body.textContent.trim().length > 0)
              )
              
              if (!hasRealContent) {
                isResolved = true
                setLoadError(true)
                setLoadSuccess(false)
                setIsChecking(false)
                setErrorMessage('The website blocked embedding due to security restrictions (X-Frame-Options or Content Security Policy)')
              } else {
                isResolved = true
                setLoadSuccess(true)
                setIsChecking(false)
              }
            } else {
              isResolved = true
              setLoadSuccess(true)
              setIsChecking(false)
            }
          } catch (e) {
            const errorMessage = e instanceof Error ? e.message : String(e)
            
            // Check for frame blocking errors which indicate the site is blocked
            if (errorMessage.toLowerCase().includes('sandbox access violation') ||
                errorMessage.toLowerCase().includes('refused to display') ||
                errorMessage.toLowerCase().includes('x-frame-options')) {
              isResolved = true
              setLoadError(true)
              setLoadSuccess(false)
              setIsChecking(false)
              setErrorMessage('The website blocked embedding due to security restrictions (X-Frame-Options or Content Security Policy)')
            } else {
              isResolved = true
              setLoadSuccess(true)
              setIsChecking(false)
            }
          } finally {
            // Clean up the test iframe
            try {
              document.body.removeChild(testIframe)
            } catch (e) {
              // Ignore cleanup errors
            }
          }
        }, 1500) // Wait a bit for content to load
      }
      
      // Handle iframe error
      testIframe.onerror = () => {
        if (isResolved) return
        isResolved = true
        setLoadError(true)
        setErrorMessage('The website could not be loaded due to network or security restrictions')
        try {
          document.body.removeChild(testIframe)
        } catch (e) {
          // Ignore cleanup errors
        }
      }
      
      document.body.appendChild(testIframe)
      
      // Additional check specifically for Chrome X-Frame-Options blocking
      // Chrome doesn't throw JS errors but prevents content loading
      setTimeout(() => {
        if (isResolved) return
        
        try {
          const doc = testIframe.contentDocument || testIframe.contentWindow?.document
          if (doc) {
            // We can access the document, check if it's really empty
            const html = doc.documentElement?.innerHTML || ''
            const bodyText = doc.body?.textContent?.trim() || ''
            const hasElements = doc.body?.children?.length || 0
            
            // If document is accessible but completely empty or minimal, it's likely blocked
            if (html.length < 100 && hasElements === 0 && !bodyText) {
              isResolved = true
              setLoadError(true)
              setLoadSuccess(false)
              setIsChecking(false)
              setErrorMessage('The website blocked embedding due to security restrictions (X-Frame-Options or Content Security Policy)')
            } else {
              isResolved = true
              setLoadSuccess(true)
              setIsChecking(false)
            }
          }
        } catch (e) {
          // This catch block will handle any access errors
          isResolved = true
          setLoadError(true)
          setLoadSuccess(false)
          setIsChecking(false)
          setErrorMessage('The website blocked embedding due to security restrictions (X-Frame-Options or Content Security Policy)')
        }
      }, 3000) // Longer timeout to be sure content had time to load
      
      // Cleanup timeout
      setTimeout(() => {
        if (!isResolved) {
          setIsChecking(false) // Stop the pulsing animation
          setLoadSuccess(true) // Assume success after timeout
          try {
            document.body.removeChild(testIframe)
          } catch (e) {
            // Ignore cleanup errors
          }
        }
      }, 5000)
    }, 1000) // Reduced initial delay

    return () => clearTimeout(checkTimer)
  }, [url, iframeKey, isEditing])


  // Test if URL can be loaded using object element
  const testUrlLoading = (testUrl: string): Promise<boolean> => {
    return new Promise((resolve) => {
      const obj = document.createElement('object')
      obj.style.position = 'absolute'
      obj.style.top = '-9999px'
      obj.style.left = '-9999px'
      obj.style.width = '1px'
      obj.style.height = '5px'
      obj.style.visibility = 'hidden'
      obj.innerHTML = '<div style="height:5px"></div>' // fallback
      obj.data = testUrl

      let isResolved = false
      let intervalCount = 0

      const isReallyLoaded = (element: HTMLObjectElement) => {
        return element.offsetHeight !== 5 // fallback height
      }

      const hasResult = (element: HTMLObjectElement) => {
        return element.offsetHeight > 0
      }

      const resolveResult = (success: boolean, reason: string) => {
        if (isResolved) return
        isResolved = true
        try {
          document.body.removeChild(obj)
        } catch (e) {
          // Ignore cleanup errors
        }
        resolve(success)
      }

      // Chrome calls always, Firefox on load
      obj.onload = () => {
        // Don't resolve immediately, let the interval checks handle it
      }

      // Firefox on error
      obj.onerror = (e) => {
        resolveResult(false, 'onerror event')
      }

      // Safari polling
      const interval = () => {
        if (isResolved) return
        
        if (hasResult(obj)) {
          if (isReallyLoaded(obj)) {
            intervalCount++
            // Give it 3 checks (1.5 seconds total) to be sure
            if (intervalCount >= 3) {
              resolveResult(true, `interval success after ${intervalCount} checks`)
            } else {
              setTimeout(interval, 500) // Check every 500ms instead of 100ms
            }
          } else {
            // Don't fail immediately - the site might still be loading
            if (intervalCount < 2) {
              intervalCount++
              setTimeout(interval, 500)
            } else {
              resolveResult(false, 'interval - confirmed not loaded after retries')
            }
          }
        } else {
          setTimeout(interval, 500)
        }
      }

      // Timeout after 4 seconds - assume it works if taking too long
      setTimeout(() => {
        resolveResult(true, '4-second timeout - assuming success')
      }, 4000)

      document.body.appendChild(obj)
      
      // Wait longer before starting checks to give it time to load
      setTimeout(() => {
        interval()
      }, 1000)
    })
  }

  // Listen for console errors that might indicate blocking
  useEffect(() => {
    const originalError = console.error
    
    const errorListener = (...args: any[]) => {
      const message = args.join(' ')
      const lowerMessage = message.toLowerCase()
      
      if (lowerMessage.includes('refused to display') || 
          lowerMessage.includes('x-frame-options') || 
          lowerMessage.includes('frame-ancestors') ||
          lowerMessage.includes('refused to load')) {
        
        let specificError = ''
        if (lowerMessage.includes('x-frame-options')) {
          specificError = 'The website blocked embedding using X-Frame-Options headers'
        } else if (lowerMessage.includes('frame-ancestors')) {
          specificError = 'The website blocked embedding using Content Security Policy frame-ancestors directive'
        } else if (lowerMessage.includes('refused to display')) {
          specificError = 'The website refused to be displayed in a frame due to security policies'
        } else {
          specificError = 'The website blocked embedding due to security restrictions'
        }
        
        setLoadError(true)
        setErrorMessage(specificError)
      }
      originalError.apply(console, args)
    }
    
    console.error = errorListener
    
    return () => {
      console.error = originalError
    }
  }, [url, iframeKey])

  const handleSave = async () => {
    try {
      const response = await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ query: url }),
      })
      
      if (response.ok) {
        onUpdate(cell.id, { query: url })
        setIsEditing(false)
        setLoadError(false)
      }
    } catch (error) {
      console.error('Failed to save webpage cell:', error)
    }
  }

  const handleCancel = () => {
    setUrl(cell.query || '')
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

  const handleRefresh = () => {
    setLoadError(false)
    setIframeKey(prev => prev + 1)
  }

  const handleIframeError = () => {
    setLoadError(true)
    setLoadSuccess(false)
    setIsChecking(false)
    setErrorMessage('The website could not be loaded in the iframe')
  }

  const handleIframeLoad = () => {
    // Don't set success immediately - wait for the verification check to complete
    // The verification process will set success or error after checking for blocking
    setIsLoading(false)
  }

  const isValidUrl = (string: string) => {
    try {
      new URL(string)
      return true
    } catch (_) {
      return false
    }
  }

  const cellClasses = [
    'cell',
    'webpage-cell',
    isEditing ? 'editing' : '',
    isDragging ? 'dragging' : '',
    isDragOver ? 'drag-over' : '',
    loadError ? 'executed-error' : '',
    loadSuccess && !loadError ? 'executed-success' : '',
    isChecking ? 'checking' : ''
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
            <Globe size={12} />
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
                {cellName || 'Webpage'}
              </span>
            )}
          </span>
          {url && <span className="cell-url">{url}</span>}
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
              {url && isValidUrl(url) && (
                <>
                  <button onClick={handleRefresh} className="cell-action refresh" title="Refresh webpage">
                    <RefreshCw size={14} />
                  </button>
                  <a
                    href={url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="cell-action external"
                    title="Open in new tab"
                  >
                    <ExternalLink size={14} />
                  </a>
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
          <div className="webpage-editor">
            <input
              type="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  handleSave()
                } else if (e.key === 'Escape') {
                  handleCancel()
                }
              }}
              placeholder="https://example.com"
              className="webpage-url-input"
              autoFocus
            />
            <div className="editor-hint">Press Enter to save, Escape to cancel</div>
          </div>
        ) : (
          <>
            {loadError && (
              <div className="cell-error">
                <strong>Error:</strong> {errorMessage || 'The website could not be loaded in the iframe'}
                {url && (
                  <div style={{ marginTop: '8px' }}>
                    <a
                      href={url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="external-link"
                    >
                      <ExternalLink size={14} />
                      Open in new tab instead
                    </a>
                  </div>
                )}
              </div>
            )}
            
            {url ? (
              isValidUrl(url) ? (
                !loadError && (
                  <div className="webpage-content">
                    <iframe
                      ref={iframeRef}
                      key={iframeKey}
                      src={url}
                      className="webpage-iframe"
                      title={cellName || 'Webpage'}
                      sandbox="allow-scripts allow-forms allow-popups allow-top-navigation"
                      onLoad={handleIframeLoad}
                      onError={handleIframeError}
                    />
                  </div>
                )
              ) : (
                <div className="cell-error">
                  <strong>Error:</strong> Invalid URL: {url}
                  <br />
                  Please enter a valid URL starting with http:// or https://
                </div>
              )
            ) : (
              <div className="cell-empty">
                <Globe size={48} className="empty-icon" />
                <p>No webpage configured</p>
                <button onClick={() => setIsEditing(true)} className="btn btn-primary">
                  Add URL
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}