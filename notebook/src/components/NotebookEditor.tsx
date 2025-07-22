import React, { useState, useEffect, useCallback, useRef } from 'react'
import { Notebook, Cell } from '../types/notebook'
import { CellComponent } from './CellComponent'
import { ArrowLeft, Plus, Search, FileText, Globe, ChevronDown, Check, X } from 'lucide-react'
import './NotebookEditor.css'

interface NotebookEditorProps {
  notebook: Notebook
  onBack: () => void
  onUpdate: (notebookId: string, name: string) => Promise<boolean>
}

export const NotebookEditor: React.FC<NotebookEditorProps> = ({
  notebook,
  onBack,
  onUpdate,
}) => {
  const [cells, setCells] = useState<Cell[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [draggedCellId, setDraggedCellId] = useState<string | null>(null)
  const [dragOverCellId, setDragOverCellId] = useState<string | null>(null)
  const autoScrollInterval = useRef<NodeJS.Timeout | null>(null)
  const isDraggingRef = useRef(false)
  const scrolledPixels = useRef(0)
  const [addCellDropdownOpen, setAddCellDropdownOpen] = useState(false)
  const [isEditingTitle, setIsEditingTitle] = useState(false)
  const [titleValue, setTitleValue] = useState(notebook.name)

  useEffect(() => {
    loadNotebook()
  }, [notebook.id])
  

  // Update title value when notebook changes
  useEffect(() => {
    setTitleValue(notebook.name)
  }, [notebook.name])

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (addCellDropdownOpen) {
        const target = event.target as Element
        if (!target.closest('.add-cell-dropdown')) {
          setAddCellDropdownOpen(false)
        }
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [addCellDropdownOpen])

  // Cleanup auto-scroll interval on unmount
  useEffect(() => {
    return () => {
      if (autoScrollInterval.current) {
        clearInterval(autoScrollInterval.current)
        autoScrollInterval.current = null
      }
    }
  }, [])

  const loadNotebook = async () => {
    try {
      setIsLoading(true)
      const response = await fetch(`/api/notebooks/${notebook.id}`)
      const data = await response.json()
      setCells(data.cells || [])
      setError(null)
    } catch (err) {
      console.error('Failed to load notebook:', err)
      setError('Failed to load notebook')
    } finally {
      setIsLoading(false)
    }
  }

  const handleAddCell = async (type: 'query' | 'markdown' | 'webpage' = 'query') => {
    try {
      const newCell = {
        type,
        query: '',
        visualization_type: 'json' as const,
        refresh_interval: 0,
        position: cells.length,
        row_index: cells.length,
        col_index: 0,
        layout_mode: 1 as const,
        config: {},
      }

      const response = await fetch(`/api/notebooks/${notebook.id}/cells`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(newCell),
      })

      if (response.ok) {
        const createdCell = await response.json()
        setCells([createdCell, ...cells])
      }
    } catch (err) {
      console.error('Failed to add cell:', err)
    }
  }

  const handleCellUpdate = useCallback((cellId: string, updates: Partial<Cell>) => {
    setCells(prevCells => {
      const index = prevCells.findIndex(cell => cell.id === cellId)
      if (index === -1) return prevCells
      
      // Only update if the cell actually changed
      const oldCell = prevCells[index]
      const newCell = { ...oldCell, ...updates }
      
      // Check if anything actually changed
      if (JSON.stringify(oldCell) === JSON.stringify(newCell)) {
        return prevCells // Return same array reference if nothing changed
      }
      
      // Create new array only if something changed
      const newCells = [...prevCells]
      newCells[index] = newCell
      return newCells
    })
  }, [])

  const handleCellDelete = async (cellId: string) => {
    try {
      const response = await fetch(`/api/notebooks/${notebook.id}/cells/${cellId}`, {
        method: 'DELETE',
      })

      if (response.ok) {
        setCells(cells.filter(cell => cell.id !== cellId))
      } else {
        console.error('Failed to delete cell')
      }
    } catch (err) {
      console.error('Failed to delete cell:', err)
    }
  }

  const startAutoScroll = (direction: 'up' | 'down') => {
    if (autoScrollInterval.current || !isDraggingRef.current) return
    
    scrolledPixels.current = 0 // Reset counter
    const baseScrollAmount = direction === 'up' ? -8 : 8
    
    autoScrollInterval.current = setInterval(() => {
      if (!isDraggingRef.current) {
        stopAutoScroll()
        return
      }
      
      // Three speed levels: 0-500px (1x), 500-1200px (2x), 1200px+ (3x)
      const scrolledDistance = Math.abs(scrolledPixels.current)
      let multiplier = 1
      if (scrolledDistance > 1200) {
        multiplier = 3
      } else if (scrolledDistance > 500) {
        multiplier = 2
      }
      const scrollAmount = baseScrollAmount * multiplier
      
      window.scrollBy(0, scrollAmount)
      scrolledPixels.current += scrollAmount
    }, 16)
  }

  const stopAutoScroll = () => {
    if (autoScrollInterval.current) {
      clearInterval(autoScrollInterval.current)
      autoScrollInterval.current = null
    }
    scrolledPixels.current = 0 // Reset counter when stopping
  }

  const checkAutoScroll = (e: DragEvent) => {
    if (!isDraggingRef.current) return
    
    const viewportHeight = window.innerHeight
    const scrollThreshold = 80
    
    if (e.clientY < scrollThreshold) {
      if (!autoScrollInterval.current) {
        startAutoScroll('up')
      }
    } else if (e.clientY > viewportHeight - scrollThreshold) {
      if (!autoScrollInterval.current) {
        startAutoScroll('down')
      }
    } else {
      stopAutoScroll()
    }
  }

  const handleDragStart = (cellId: string) => {
    setDraggedCellId(cellId)
    isDraggingRef.current = true
    
    // Add global drag event listeners for auto-scroll
    const handleGlobalDragOver = (e: DragEvent) => {
      checkAutoScroll(e)
    }
    
    const handleGlobalDragEnd = () => {
      isDraggingRef.current = false
      stopAutoScroll()
      document.removeEventListener('dragover', handleGlobalDragOver)
      document.removeEventListener('dragend', handleGlobalDragEnd)
    }
    
    document.addEventListener('dragover', handleGlobalDragOver)
    document.addEventListener('dragend', handleGlobalDragEnd)
  }

  const handleDragEnd = () => {
    setDraggedCellId(null)
    setDragOverCellId(null)
    isDraggingRef.current = false
    stopAutoScroll()
  }

  const handleDragOver = (cellId: string) => {
    setDragOverCellId(cellId)
  }

  const handleDrop = async (targetCellId: string) => {
    if (!draggedCellId || draggedCellId === targetCellId) {
      setDraggedCellId(null)
      setDragOverCellId(null)
      return
    }

    const draggedIndex = cells.findIndex(cell => cell.id === draggedCellId)
    const targetIndex = cells.findIndex(cell => cell.id === targetCellId)
    
    if (draggedIndex === -1 || targetIndex === -1) {
      setDraggedCellId(null)
      setDragOverCellId(null)
      return
    }

    // Create new cell order
    const newCells = [...cells]
    const [draggedCell] = newCells.splice(draggedIndex, 1)
    newCells.splice(targetIndex, 0, draggedCell)

    // Update positions
    const updatedCells = newCells.map((cell, index) => ({
      ...cell,
      position: index,
      row_index: index
    }))

    // Optimistically update UI
    setCells(updatedCells)

    // Update backend
    try {
      const response = await fetch(`/api/notebooks/${notebook.id}/cells/reorder`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          cell_orders: updatedCells.map(cell => ({
            id: cell.id,
            position: cell.position
          }))
        }),
      })

      if (!response.ok) {
        console.error('Failed to update cell order on server')
        // Revert to original order
        loadNotebook()
      }
    } catch (err) {
      console.error('Failed to update cell order:', err)
      // Revert to original order
      loadNotebook()
    }

    setDraggedCellId(null)
    setDragOverCellId(null)
  }

  const handleTitleEdit = () => {
    setIsEditingTitle(true)
  }

  const handleTitleSave = async () => {
    if (titleValue.trim() !== notebook.name && titleValue.trim() !== '') {
      const success = await onUpdate(notebook.id, titleValue.trim())
      if (success) {
        setIsEditingTitle(false)
      } else {
        // Revert to original name on error
        setTitleValue(notebook.name)
      }
    } else {
      setIsEditingTitle(false)
      setTitleValue(notebook.name) // Revert if empty or unchanged
    }
  }

  const handleTitleCancel = () => {
    setTitleValue(notebook.name)
    setIsEditingTitle(false)
  }

  const handleTitleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleTitleSave()
    } else if (e.key === 'Escape') {
      handleTitleCancel()
    }
  }

  if (isLoading) {
    return (
      <div className="notebook-editor">
        <div className="loading">Loading notebook...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="notebook-editor">
        <div className="error">{error}</div>
        <button onClick={onBack}>Back to Notebooks</button>
      </div>
    )
  }

  return (
    <div className="notebook-editor">
      <div className="notebook-header">
        <button className="back-button" onClick={onBack}>
          <ArrowLeft size={20} />
        </button>
        <div className="notebook-title-container">
          {isEditingTitle ? (
            <div className="title-edit-group">
              <input
                type="text"
                value={titleValue}
                onChange={(e) => setTitleValue(e.target.value)}
                onKeyDown={handleTitleKeyDown}
                onBlur={handleTitleSave}
                className="notebook-title-input"
                autoFocus
                maxLength={100}
              />
              <button className="title-action-btn save" onClick={handleTitleSave}>
                <Check size={16} />
              </button>
              <button className="title-action-btn cancel" onClick={handleTitleCancel}>
                <X size={16} />
              </button>
            </div>
          ) : (
            <div className="title-display-group">
              <h1 
                className="notebook-title clickable" 
                onClick={handleTitleEdit}
                title="Click to rename"
              >
                {notebook.name}
              </h1>
            </div>
          )}
        </div>
        <div className="notebook-actions">
          <div className="add-cell-dropdown">
            <button 
              className="btn btn-primary"
              onClick={() => setAddCellDropdownOpen(!addCellDropdownOpen)}
            >
              <Plus size={16} />
              Add Cell
            </button>
            {addCellDropdownOpen && (
              <div className="add-cell-options">
                <button 
                  onClick={() => {
                    handleAddCell('query')
                    setAddCellDropdownOpen(false)
                  }}
                  className="add-cell-option"
                >
                  <Search size={16} />
                  Query
                </button>
                <button 
                  onClick={() => {
                    handleAddCell('markdown')
                    setAddCellDropdownOpen(false)
                  }}
                  className="add-cell-option"
                >
                  <FileText size={16} />
                  Markdown
                </button>
                <button 
                  onClick={() => {
                    handleAddCell('webpage')
                    setAddCellDropdownOpen(false)
                  }}
                  className="add-cell-option"
                >
                  <Globe size={16} />
                  Webpage
                </button>
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="notebook-content">
        {cells.length === 0 ? (
          <div className="notebook-empty">
            <h3>Empty Notebook</h3>
            <p>Add your first cell to start building your notebook.</p>
            <button 
              className="btn btn-primary"
              onClick={() => handleAddCell('query')}
            >
              Add Query Cell
            </button>
          </div>
        ) : (
          <div className="notebook-cells">
            {cells.map((cell, index) => {
              return (
                <CellComponent
                  key={cell.id}
                  cell={cell}
                  onUpdate={handleCellUpdate}
                  onDelete={handleCellDelete}
                  onDragStart={handleDragStart}
                  onDragEnd={handleDragEnd}
                  onDragOver={handleDragOver}
                  onDrop={handleDrop}
                  isDragging={draggedCellId === cell.id}
                  isDragOver={dragOverCellId === cell.id}
                />
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}