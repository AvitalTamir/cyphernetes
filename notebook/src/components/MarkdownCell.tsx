import React, { useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Cell } from '../types/notebook'
import { Edit3, Eye, Save, X, Trash2, GripVertical, FileText } from 'lucide-react'
import { SyntaxHighlighter } from './SyntaxHighlighter'
import './MarkdownCell.css'

interface MarkdownCellProps {
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

export const MarkdownCell: React.FC<MarkdownCellProps> = ({
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
  const [content, setContent] = useState(cell.query || '')

  const handleSave = async () => {
    try {
      const response = await fetch(`/api/notebooks/${cell.notebook_id}/cells/${cell.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ query: content }),
      })
      
      if (response.ok) {
        onUpdate(cell.id, { query: content })
        setIsEditing(false)
      }
    } catch (error) {
      console.error('Failed to save markdown cell:', error)
    }
  }

  const handleCancel = () => {
    setContent(cell.query || '')
    setIsEditing(false)
  }

  const toggleMode = () => {
    if (isEditing) {
      handleSave()
    } else {
      setIsEditing(true)
    }
  }

  const handleDragStart = (e: React.DragEvent) => {
    e.dataTransfer.setData('text/plain', cell.id)
    e.dataTransfer.effectAllowed = 'move'
    
    // Find the cell container for the drag image
    const cellElement = (e.target as HTMLElement).closest('.markdown-cell')
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

  return (
    <div 
      className={`markdown-cell ${isEditing ? 'editing' : 'preview'} ${isDragging ? 'dragging' : ''} ${isDragOver ? 'drag-over' : ''}`}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
    >
      <div className="markdown-cell-header">
        <div className="cell-info">
          <div 
            className="drag-handle" 
            title="Drag to reorder"
            draggable={!isEditing}
            onDragStart={handleDragStart}
            onDragEnd={handleDragEnd}
          >
            <GripVertical size={14} />
          </div>
          <span className="cell-type">
            <FileText size={12} />
            Markdown
          </span>
          <span className="cell-mode">{isEditing ? 'Editing' : 'Preview'}</span>
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

      <div className="markdown-cell-content">
        {isEditing ? (
          <textarea
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder="Enter your markdown content here...

# Heading 1
## Heading 2

**Bold text** and *italic text*

- List item 1
- List item 2

[Link](https://example.com)

```javascript
// Code block
console.log('Hello, world!');
```

| Column 1 | Column 2 |
|----------|----------|
| Cell 1   | Cell 2   |"
            className="markdown-editor"
            rows={12}
          />
        ) : (
          <div className="markdown-preview">
            {content ? (
              <ReactMarkdown 
                remarkPlugins={[remarkGfm]}
                components={{
                  // Customize code blocks
                  code: ({className, children, ...props}) => {
                    const match = /language-(\w+)/.exec(className || '')
                    const language = match?.[1]
                    const isInline = !match
                    
                    if (isInline) {
                      return (
                        <code className="inline-code" {...props}>
                          {children}
                        </code>
                      )
                    }
                    
                    // For supported languages, use syntax highlighter
                    if (language === 'json' || language === 'yaml') {
                      return (
                        <SyntaxHighlighter
                          code={String(children).replace(/\n$/, '')}
                          language={language as 'json' | 'yaml'}
                        />
                      )
                    }
                    
                    // For other languages, use the themed code block
                    return (
                      <pre className="syntax-highlighter catppuccin-latte">
                        <code className={className} {...props}>
                          {children}
                        </code>
                      </pre>
                    )
                  },
                  // Customize tables
                  table: ({children}) => (
                    <div className="table-wrapper">
                      <table className="markdown-table">{children}</table>
                    </div>
                  ),
                  // Customize links
                  a: ({href, children}) => (
                    <a href={href} target="_blank" rel="noopener noreferrer">
                      {children}
                    </a>
                  )
                }}
              >
                {content}
              </ReactMarkdown>
            ) : (
              <div className="empty-markdown">
                <p>This markdown cell is empty.</p>
                <button onClick={() => setIsEditing(true)} className="btn btn-outline">
                  <Edit3 size={16} />
                  Add Content
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}