import React, { useState } from 'react'
import { Notebook } from '../types/notebook'
import { Plus, Trash2, Users } from 'lucide-react'
import './NotebookList.css'

interface NotebookListProps {
  notebooks: Notebook[]
  onSelect: (notebook: Notebook) => void
  onCreate: (name: string) => void
  onDelete: (notebookId: string) => void
}

export const NotebookList: React.FC<NotebookListProps> = ({
  notebooks,
  onSelect,
  onCreate,
  onDelete,
}) => {
  const [isCreating, setIsCreating] = useState(false)
  const [newNotebookName, setNewNotebookName] = useState('')

  const handleCreate = () => {
    if (newNotebookName.trim()) {
      onCreate(newNotebookName.trim())
      setNewNotebookName('')
      setIsCreating(false)
    }
  }

  const handleCancel = () => {
    setNewNotebookName('')
    setIsCreating(false)
  }

  const handleDelete = (e: React.MouseEvent, notebookId: string, notebookName: string) => {
    e.stopPropagation() // Prevent notebook selection when clicking delete
    if (window.confirm(`Are you sure you want to delete the notebook "${notebookName}"? This action cannot be undone.`)) {
      onDelete(notebookId)
    }
  }

  return (
    <div className="notebook-list">
      <div className="notebook-list-header">
        <h2>Your Notebooks</h2>
        <button
          className="btn btn-primary"
          onClick={() => setIsCreating(true)}
        >
          <Plus size={16} />
          New Notebook
        </button>
      </div>

      {isCreating && (
        <div className="notebook-create-form">
          <input
            type="text"
            placeholder="Enter notebook name..."
            value={newNotebookName}
            onChange={(e) => setNewNotebookName(e.target.value)}
            onKeyPress={(e) => e.key === 'Enter' && handleCreate()}
            autoFocus
          />
          <div className="notebook-create-actions">
            <button className="btn btn-primary" onClick={handleCreate}>
              Create
            </button>
            <button className="btn btn-secondary" onClick={handleCancel}>
              Cancel
            </button>
          </div>
        </div>
      )}

      <div className="notebook-grid">
        {notebooks.map((notebook) => (
          <div
            key={notebook.id}
            className="notebook-card"
            onClick={() => onSelect(notebook)}
          >
            <div className="notebook-card-header">
              <h3 className="notebook-card-title">{notebook.name}</h3>
              <div className="notebook-card-actions">
                <div className="notebook-card-date">
                  {new Date(notebook.updated_at).toLocaleDateString()}
                </div>
                <button
                  className="btn btn-danger btn-small"
                  onClick={(e) => handleDelete(e, notebook.id, notebook.name)}
                  title="Delete notebook"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            </div>
            <div className="notebook-card-info">
              <span className="notebook-card-cells">
                {notebook.cells?.length || 0} cells
              </span>
              {notebook.shared_with.length > 0 && (
                <span className="notebook-card-shared">
                  <Users size={14} />
                  Shared with {notebook.shared_with.length} users
                </span>
              )}
            </div>
          </div>
        ))}
        
        {notebooks.length === 0 && !isCreating && (
          <div className="notebook-empty">
            <h3>No notebooks yet</h3>
            <p>Create your first notebook to get started with Cyphernetes queries.</p>
          </div>
        )}
      </div>
    </div>
  )
}