import React, { useState } from 'react'
import { Cell } from '../types/notebook'
import { ScrollText, Edit3, Trash2, Save, X } from 'lucide-react'
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
  const [isEditing, setIsEditing] = useState(false)
  const [podName, setPodName] = useState(cell.query || '')
  const [isEditingName, setIsEditingName] = useState(false)
  const [cellName, setCellName] = useState(cell.name || '')

  const handleSave = () => {
    onUpdate(cell.id, { query: podName })
    setIsEditing(false)
  }

  const handleNameSave = () => {
    onUpdate(cell.id, { name: cellName })
    setIsEditingName(false)
  }

  const handleDragStart = (e: React.DragEvent) => {
    e.dataTransfer.effectAllowed = 'move'
    onDragStart?.(cell.id)
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

  const cellClasses = [
    'cell',
    isDragging ? 'dragging' : '',
    isDragOver ? 'drag-over' : ''
  ].filter(Boolean).join(' ')

  if (isEditing) {
    return (
      <div className={cellClasses}>
        <div className="cell-header">
          <div className="cell-type-indicator logs">
            <ScrollText size={16} />
            <span>Logs</span>
          </div>
        </div>
        <div className="cell-edit">
          <input
            type="text"
            value={podName}
            onChange={(e) => setPodName(e.target.value)}
            placeholder="Pod name (e.g., nginx-deployment-abc123)"
            className="cell-edit-input"
            autoFocus
          />
          <div className="cell-edit-actions">
            <button onClick={handleSave} className="cell-action save">
              <Save size={14} />
              Save
            </button>
            <button onClick={() => setIsEditing(false)} className="cell-action cancel">
              <X size={14} />
              Cancel
            </button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div
      className={cellClasses}
      draggable
      onDragStart={handleDragStart}
      onDragEnd={onDragEnd}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
    >
      <div className="cell-header">
        <div className="cell-type-indicator logs">
          <ScrollText size={16} />
          <span>Logs</span>
        </div>
        <div className="cell-name-section">
          {isEditingName ? (
            <div className="cell-name-edit">
              <input
                type="text"
                value={cellName}
                onChange={(e) => setCellName(e.target.value)}
                onBlur={handleNameSave}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleNameSave()
                  if (e.key === 'Escape') {
                    setCellName(cell.name || '')
                    setIsEditingName(false)
                  }
                }}
                className="cell-name-input"
                placeholder="Cell name"
                autoFocus
              />
            </div>
          ) : (
            <div className="cell-name-display" onClick={() => setIsEditingName(true)}>
              {cell.name || 'Unnamed logs cell'}
            </div>
          )}
        </div>
        <div className="cell-actions">
          <button onClick={() => setIsEditing(true)} className="cell-action edit">
            <Edit3 size={14} />
            Edit
          </button>
          <button onClick={() => onDelete(cell.id)} className="cell-action delete">
            <Trash2 size={14} />
            Delete
          </button>
        </div>
      </div>

      <div className="cell-content">
        {podName ? (
          <div className="logs-content">
            <div className="logs-info">
              <strong>Pod:</strong> {podName}
            </div>
            <div className="logs-placeholder">
              Logs functionality coming soon...
            </div>
          </div>
        ) : (
          <div className="cell-empty">
            <p>Click "Edit" to specify a pod name for log viewing</p>
          </div>
        )}
      </div>
    </div>
  )
}