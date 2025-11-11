import React from 'react'
import './SyntaxHighlighter.css'

interface SyntaxHighlighterProps {
  code: string
  language: 'json' | 'yaml'
}

export const SyntaxHighlighter: React.FC<SyntaxHighlighterProps> = ({ code, language }) => {
  const highlightJSON = (jsonString: string): React.ReactNode => {
    try {
      // Parse and re-stringify to ensure valid JSON and proper formatting
      const parsed = JSON.parse(jsonString)
      const formatted = JSON.stringify(parsed, null, 2)
      
      return highlightJSONTokens(formatted)
    } catch (error) {
      // If parsing fails, still try to highlight what we can
      return highlightJSONTokens(jsonString)
    }
  }

  const highlightJSONTokens = (jsonString: string): React.ReactNode => {
    // Process the string character by character to avoid overlapping matches
    const result: React.ReactNode[] = []
    let currentIndex = 0
    let keyIndex = 0

    while (currentIndex < jsonString.length) {
      const remaining = jsonString.slice(currentIndex)
      
      // Skip whitespace
      const whitespaceMatch = remaining.match(/^(\s+)/)
      if (whitespaceMatch) {
        result.push(<span key={`ws-${keyIndex++}`} className="catppuccin-fg">{whitespaceMatch[1]}</span>)
        currentIndex += whitespaceMatch[1].length
        continue
      }

      // Match strings (including property names and values)
      const stringMatch = remaining.match(/^("(?:[^"\\]|\\.)*")/)
      if (stringMatch) {
        result.push(<span key={`str-${keyIndex++}`} className="catppuccin-string">{stringMatch[1]}</span>)
        currentIndex += stringMatch[1].length
        continue
      }

      // Match numbers
      const numberMatch = remaining.match(/^(-?\d+\.?\d*(?:[eE][+-]?\d+)?)/)
      if (numberMatch) {
        result.push(<span key={`num-${keyIndex++}`} className="catppuccin-number">{numberMatch[1]}</span>)
        currentIndex += numberMatch[1].length
        continue
      }

      // Match booleans
      const booleanMatch = remaining.match(/^(true|false)/)
      if (booleanMatch) {
        result.push(<span key={`bool-${keyIndex++}`} className="catppuccin-boolean">{booleanMatch[1]}</span>)
        currentIndex += booleanMatch[1].length
        continue
      }

      // Match null
      const nullMatch = remaining.match(/^(null)/)
      if (nullMatch) {
        result.push(<span key={`null-${keyIndex++}`} className="catppuccin-null">{nullMatch[1]}</span>)
        currentIndex += nullMatch[1].length
        continue
      }

      // Match colon
      if (remaining.startsWith(':')) {
        result.push(<span key={`colon-${keyIndex++}`} className="catppuccin-colon">:</span>)
        currentIndex += 1
        continue
      }

      // Match punctuation
      const punctuationMatch = remaining.match(/^([{}[\],])/)
      if (punctuationMatch) {
        result.push(<span key={`punct-${keyIndex++}`} className="catppuccin-punctuation">{punctuationMatch[1]}</span>)
        currentIndex += 1
        continue
      }

      // Default: single character
      result.push(<span key={`char-${keyIndex++}`} className="catppuccin-fg">{remaining[0]}</span>)
      currentIndex += 1
    }

    return result
  }

  const highlightYAML = (yamlString: string): React.ReactNode => {
    const lines = yamlString.split('\n')
    
    return lines.map((line, lineIndex) => {
      const highlightedLine = highlightYAMLLine(line)
      return (
        <div key={lineIndex}>
          {highlightedLine}
          {lineIndex < lines.length - 1 && '\n'}
        </div>
      )
    })
  }

  const highlightYAMLLine = (line: string): React.ReactNode => {
    // YAML patterns
    const commentMatch = line.match(/^(\s*)(#.*)$/)
    if (commentMatch) {
      return (
        <>
          <span className="catppuccin-fg">{commentMatch[1]}</span>
          <span className="catppuccin-comment">{commentMatch[2]}</span>
        </>
      )
    }

    // Key-value pairs
    const keyValueMatch = line.match(/^(\s*)([^:]+)(:)(\s*)(.*)$/)
    if (keyValueMatch) {
      const [, indent, key, colon, spacing, value] = keyValueMatch
      return (
        <>
          <span className="catppuccin-fg">{indent}</span>
          <span className="catppuccin-key">{key}</span>
          <span className="catppuccin-colon">{colon}</span>
          <span className="catppuccin-fg">{spacing}</span>
          {highlightYAMLValue(value)}
        </>
      )
    }

    // List items
    const listMatch = line.match(/^(\s*)([-*])(\s+)(.*)$/)
    if (listMatch) {
      const [, indent, bullet, spacing, content] = listMatch
      return (
        <>
          <span className="catppuccin-fg">{indent}</span>
          <span className="catppuccin-punctuation">{bullet}</span>
          <span className="catppuccin-fg">{spacing}</span>
          {highlightYAMLValue(content)}
        </>
      )
    }

    // Document separators
    if (line.match(/^---\s*$/) || line.match(/^\.\.\.\s*$/)) {
      return <span className="catppuccin-punctuation">{line}</span>
    }

    // Default
    return <span className="catppuccin-fg">{line}</span>
  }

  const highlightYAMLValue = (value: string): React.ReactNode => {
    // String values (quoted)
    if (value.match(/^["'].*["']$/)) {
      return <span className="catppuccin-string">{value}</span>
    }

    // Numbers
    if (value.match(/^-?\d+\.?\d*$/)) {
      return <span className="catppuccin-number">{value}</span>
    }

    // Booleans
    if (value.match(/^(true|false|yes|no|on|off)$/i)) {
      return <span className="catppuccin-boolean">{value}</span>
    }

    // Null values
    if (value.match(/^(null|~)$/i)) {
      return <span className="catppuccin-null">{value}</span>
    }

    // Arrays/objects indicators
    if (value.match(/^[\[{]/) || value === '|' || value === '>') {
      return <span className="catppuccin-punctuation">{value}</span>
    }

    // Default unquoted string
    return <span className="catppuccin-string">{value}</span>
  }

  return (
    <pre className="syntax-highlighter catppuccin-latte">
      <code>
        {language === 'json' ? highlightJSON(code) : highlightYAML(code)}
      </code>
    </pre>
  )
}