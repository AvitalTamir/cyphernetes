import React, { useState, useCallback, useEffect, useRef } from 'react'
import { Prism as PrismSyntaxHighlighter } from 'react-syntax-highlighter'
import { oneLight } from 'react-syntax-highlighter/dist/cjs/styles/prism'

// Debounce function
function debounce<F extends (...args: any[]) => any>(func: F, wait: number): (...args: Parameters<F>) => void {
  let timeoutId: ReturnType<typeof setTimeout> | null = null
  return (...args: Parameters<F>) => {
    if (timeoutId !== null) {
      clearTimeout(timeoutId)
    }
    timeoutId = setTimeout(() => func(...args), wait)
  }
}

interface CyphernetesEditorProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  rows?: number
  readOnly?: boolean
  showSyntaxHighlighting?: boolean
  showAutocomplete?: boolean
}

export const CyphernetesEditor: React.FC<CyphernetesEditorProps> = ({
  value,
  onChange,
  placeholder = "Enter your Cyphernetes query...",
  rows = 4,
  readOnly = false,
  showSyntaxHighlighting = true,
  showAutocomplete = true
}) => {
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [cursorPosition, setCursorPosition] = useState(0)
  const [selectedSuggestionIndex, setSelectedSuggestionIndex] = useState(-1)
  const [suggestionsPosition, setSuggestionsPosition] = useState({ top: 0, left: 0 })
  
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const suggestionsRef = useRef<HTMLDivElement>(null)

  // Autocompletion functions
  const updateSuggestionsPosition = useCallback(() => {
    if (textareaRef.current) {
      const textBeforeCursor = textareaRef.current.value.substring(0, cursorPosition)
      const lines = textBeforeCursor.split('\n')
      const currentLineNumber = lines.length
      const currentLineText = lines[lines.length - 1]

      const lineHeight = 21
      const charWidth = 8.4

      const top = ((currentLineNumber - 1) * lineHeight) + 15
      const left = (currentLineText.length * charWidth) + 14

      setSuggestionsPosition({ top, left })
    }
  }, [cursorPosition])

  useEffect(() => {
    updateSuggestionsPosition()
  }, [cursorPosition, updateSuggestionsPosition])

  // Debounced autocomplete function
  const debouncedFetchSuggestions = useCallback(
    debounce(async (query: string, position: number) => {
      try {
        const response = await fetch(`/api/autocomplete?query=${encodeURIComponent(query)}&position=${position}`)
        if (response.ok) {
          const data = await response.json()
          const uniqueSuggestions = Array.from(new Set(data.suggestions || [])) as string[]
          setSuggestions(uniqueSuggestions)
          setSelectedSuggestionIndex(-1)
        }
      } catch (error) {
        console.error('Failed to fetch suggestions:', error)
      }
    }, 300),
    []
  )

  useEffect(() => {
    if (showAutocomplete && !readOnly && value.length > 0) {
      debouncedFetchSuggestions(value, cursorPosition)
    } else {
      setSuggestions([])
    }
  }, [value, cursorPosition, debouncedFetchSuggestions, showAutocomplete, readOnly])

  const insertSuggestion = useCallback((suggestion: string) => {
    const newValue = value.slice(0, cursorPosition) + suggestion + value.slice(cursorPosition)
    const newCursorPosition = cursorPosition + suggestion.length

    onChange(newValue)
    setCursorPosition(newCursorPosition)
    setSuggestions([])
    setSelectedSuggestionIndex(-1)

    // Update the cursor position in the textarea
    if (textareaRef.current) {
      textareaRef.current.setSelectionRange(newCursorPosition, newCursorPosition)
    }
  }, [value, cursorPosition, onChange])

  const scrollSuggestionIntoView = useCallback((index: number) => {
    if (suggestionsRef.current) {
      const suggestionItems = suggestionsRef.current.getElementsByClassName('suggestion-item')
      if (suggestionItems[index]) {
        suggestionItems[index].scrollIntoView({
          behavior: 'smooth',
          block: 'nearest',
        })
      }
    }
  }, [])

  const handleQueryChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const newValue = e.target.value
    const newPosition = e.target.selectionStart
    onChange(newValue)
    setCursorPosition(newPosition)
  }, [onChange])

  const handleCursorChange = useCallback(() => {
    if (textareaRef.current) {
      setCursorPosition(textareaRef.current.selectionStart)
    }
  }, [])

  const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (suggestions.length > 0) {
      switch (e.key) {
        case 'ArrowDown':
          e.preventDefault()
          const nextIndex = selectedSuggestionIndex < suggestions.length - 1 
            ? selectedSuggestionIndex + 1 
            : 0
          setSelectedSuggestionIndex(nextIndex)
          scrollSuggestionIntoView(nextIndex)
          break
          
        case 'ArrowUp':
          e.preventDefault()
          const prevIndex = selectedSuggestionIndex > 0 
            ? selectedSuggestionIndex - 1 
            : suggestions.length - 1
          setSelectedSuggestionIndex(prevIndex)
          scrollSuggestionIntoView(prevIndex)
          break
          
        case 'Enter':
        case 'Tab':
          if (selectedSuggestionIndex >= 0) {
            e.preventDefault()
            insertSuggestion(suggestions[selectedSuggestionIndex])
          }
          break
          
        case 'Escape':
          setSuggestions([])
          setSelectedSuggestionIndex(-1)
          break
      }
    }
  }, [suggestions, selectedSuggestionIndex, insertSuggestion, scrollSuggestionIntoView])

  const handleSuggestionClick = useCallback((e: React.MouseEvent, suggestion: string) => {
    e.preventDefault()
    e.stopPropagation()
    insertSuggestion(suggestion)
    if (textareaRef.current) {
      textareaRef.current.focus()
    }
  }, [insertSuggestion])

  if (readOnly && showSyntaxHighlighting) {
    return (
      <div className="cyphernetes-editor-readonly">
        <PrismSyntaxHighlighter
          language="cypher"
          style={oneLight}
          customStyle={{
            background: 'transparent',
            padding: '12px',
            margin: 0,
            fontSize: '13px',
            lineHeight: '1.5',
            borderRadius: '6px',
            border: '1px solid #e0e4e7',
          }}
        >
          {value || placeholder}
        </PrismSyntaxHighlighter>
      </div>
    )
  }

  return (
    <div className="cyphernetes-editor-container" style={{ position: 'relative' }}>
      <textarea
        ref={textareaRef}
        value={value}
        onChange={handleQueryChange}
        onKeyDown={handleKeyDown}
        onSelect={handleCursorChange}
        placeholder={placeholder}
        rows={rows}
        readOnly={readOnly}
        className="cyphernetes-editor-textarea"
        style={{
          width: '100%',
          padding: '12px',
          fontSize: '13px',
          lineHeight: '1.5',
          fontFamily: '"SF Mono", "Monaco", "Inconsolata", "Roboto Mono", "Source Code Pro", monospace',
          border: '1px solid #e0e4e7',
          borderRadius: '6px',
          resize: 'vertical',
          outline: 'none',
        }}
      />
      
      {showAutocomplete && suggestions.length > 0 && (
        <div
          ref={suggestionsRef}
          className="cyphernetes-editor-suggestions"
          style={{
            position: 'absolute',
            top: suggestionsPosition.top,
            left: suggestionsPosition.left,
            zIndex: 1000,
            backgroundColor: 'white',
            border: '1px solid #e0e4e7',
            borderRadius: '6px',
            boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
            maxHeight: '200px',
            overflowY: 'auto',
            minWidth: '120px'
          }}
        >
          {suggestions.map((suggestion, index) => (
            <div
              key={index}
              className={`suggestion-item ${index === selectedSuggestionIndex ? 'selected' : ''}`}
              onClick={(e) => handleSuggestionClick(e, suggestion)}
              style={{
                padding: '8px 12px',
                cursor: 'pointer',
                fontSize: '12px',
                fontFamily: 'monospace',
                backgroundColor: index === selectedSuggestionIndex ? '#f0f4f8' : 'transparent',
                borderBottom: index < suggestions.length - 1 ? '1px solid #f0f0f0' : 'none'
              }}
              onMouseEnter={() => setSelectedSuggestionIndex(index)}
            >
              {suggestion}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}