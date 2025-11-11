import React, { Component, ErrorInfo, ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  error?: Error
}

export class HooksErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false
  }

  public static getDerivedStateFromError(error: Error): State {
    // Check if this is likely a query parser/syntax error that should display normally
    const isParserError = error.message.toLowerCase().includes('parse') ||
                         error.message.toLowerCase().includes('syntax') ||
                         error.message.toLowerCase().includes('unexpected') ||
                         error.message.toLowerCase().includes('expected')
    
    // If it's a parser error, let it through to display normally
    if (isParserError) {
      throw error
    }
    
    // Otherwise, catch all other errors as they're likely React-related
    return { hasError: true, error }
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('HooksErrorBoundary caught a React hooks error:', error, errorInfo)
  }

  public render() {
    if (this.state.hasError) {
      return (
        <div style={{ 
          padding: '16px', 
          border: '1px solid var(--color-error, #e84393)', 
          borderRadius: '8px', 
          backgroundColor: 'rgba(232, 67, 147, 0.1)',
          margin: '10px 0',
          color: 'var(--color-text, #2d3436)'
        }}>
          <h4 style={{ margin: '0 0 8px 0', color: 'var(--color-error, #e84393)' }}>
            Cell Error
          </h4>
          <p style={{ margin: '0 0 12px 0', fontSize: '14px' }}>
            This cell encountered a React error. This is usually temporary.
          </p>
          <button 
            onClick={() => this.setState({ hasError: false, error: undefined })}
            style={{
              padding: '6px 12px',
              backgroundColor: 'var(--color-primary, #6c5ce7)',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
              fontSize: '14px'
            }}
          >
            Retry Cell
          </button>
        </div>
      )
    }

    return this.props.children
  }
}