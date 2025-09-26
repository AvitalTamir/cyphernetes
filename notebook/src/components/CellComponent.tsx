import React from 'react'
import { Cell } from '../types/notebook'
import { MarkdownCell } from './MarkdownCell'
import { QueryCell } from './QueryCell'
import { WebpageCell } from './WebpageCell'
import { HooksErrorBoundary } from './HooksErrorBoundary'

interface CellComponentProps {
  cell: Cell
  onUpdate?: (cellId: string, updates: Partial<Cell>) => void
  onDelete?: (cellId: string) => void
  onDragStart?: (cellId: string) => void
  onDragEnd?: () => void
  onDragOver?: (cellId: string) => void
  onDrop?: (cellId: string) => void
  isDragging?: boolean
  isDragOver?: boolean
  isSharedMode?: boolean
}

// Pure dispatcher component with no hooks
const CellComponentImpl: React.FC<CellComponentProps> = (props) => {
  // Provide no-op functions for undefined handlers (shared mode)
  const cellProps = {
    ...props,
    onUpdate: props.onUpdate || (() => {}),
    onDelete: props.onDelete || (() => {}),
  }

  return (
    <HooksErrorBoundary>
      {props.cell.type === 'markdown' ? (
        <MarkdownCell {...cellProps} />
      ) : props.cell.type === 'webpage' ? (
        <WebpageCell {...cellProps} />
      ) : (
        <QueryCell {...cellProps} />
      )}
    </HooksErrorBoundary>
  )
}

// Ultra-simple memo for performance
export const CellComponent = React.memo(CellComponentImpl, (prevProps, nextProps) => {
  // For polling cells, only check structural properties to prevent cascading re-renders
  if (nextProps.cell.refresh_interval > 0) {
    return (
      prevProps.cell.id === nextProps.cell.id &&
      prevProps.cell.type === nextProps.cell.type &&
      prevProps.cell.query === nextProps.cell.query &&
      prevProps.isDragging === nextProps.isDragging &&
      prevProps.isDragOver === nextProps.isDragOver
    )
  }
  
  // For non-polling cells, do full comparison
  return (
    prevProps.cell.id === nextProps.cell.id &&
    prevProps.cell.type === nextProps.cell.type &&
    prevProps.cell.last_executed === nextProps.cell.last_executed &&
    prevProps.cell.is_running === nextProps.cell.is_running &&
    JSON.stringify(prevProps.cell.results) === JSON.stringify(nextProps.cell.results) &&
    JSON.stringify(prevProps.cell.config) === JSON.stringify(nextProps.cell.config) &&
    prevProps.cell.query === nextProps.cell.query &&
    prevProps.cell.error === nextProps.cell.error &&
    prevProps.cell.refresh_interval === nextProps.cell.refresh_interval &&
    prevProps.isDragging === nextProps.isDragging &&
    prevProps.isDragOver === nextProps.isDragOver
  )
})