import React, { useState, useEffect } from 'react'
import { Notebook, Cell } from '../types/notebook'
import { CellComponent } from './CellComponent'
import { ArrowLeft, Plus, Search, FileText } from 'lucide-react'
import './NotebookEditor.css'

interface NotebookEditorProps {
  notebook: Notebook
  onBack: () => void
}

export const NotebookEditor: React.FC<NotebookEditorProps> = ({
  notebook,
  onBack,
}) => {
  const [cells, setCells] = useState<Cell[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [draggedCellId, setDraggedCellId] = useState<string | null>(null)
  const [dragOverCellId, setDragOverCellId] = useState<string | null>(null)

  useEffect(() => {
    loadNotebook()
  }, [notebook.id])

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

  const handleAddCell = async (type: 'query' | 'markdown' = 'query') => {
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
        setCells([...cells, createdCell])
      }
    } catch (err) {
      console.error('Failed to add cell:', err)
    }
  }

  const handleCellUpdate = (cellId: string, updates: Partial<Cell>) => {
    setCells(prevCells => prevCells.map(cell => 
      cell.id === cellId ? { ...cell, ...updates } : cell
    ))
  }

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

  const handleDragStart = (cellId: string) => {
    setDraggedCellId(cellId)
  }

  const handleDragEnd = () => {
    setDraggedCellId(null)
    setDragOverCellId(null)
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
          <ArrowLeft size={16} />
          Back to Notebooks
        </button>
        <h1 className="notebook-title">{notebook.name}</h1>
        <div className="notebook-actions">
          <button 
            className="btn btn-outline"
            onClick={() => handleAddCell('query')}
          >
            <Search size={16} />
            Query Cell
          </button>
          <button 
            className="btn btn-outline"
            onClick={() => handleAddCell('markdown')}
          >
            <FileText size={16} />
            Markdown Cell
          </button>
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
            {cells.map((cell) => (
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
            ))}
          </div>
        )}
      </div>
    </div>
  )
}