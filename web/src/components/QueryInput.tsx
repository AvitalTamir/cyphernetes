import React, { useState, useRef, KeyboardEvent, useEffect, useCallback } from 'react';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { dracula } from 'react-syntax-highlighter/dist/cjs/styles/prism';
import { fetchAutocompleteSuggestions } from '../api/queryApi';
import HistoryModal from './HistoryModal';
import './QueryInput.css';

interface QueryInputProps {
  onSubmit: (query: string, selectedText: string | null) => void;
  isLoading: boolean;
  queryStatus: {
    numQueries: number;
    status: 'succeeded' | 'failed';
    time: number;
  } | null;
  isHistoryModalOpen: boolean;
  setIsHistoryModalOpen: (isOpen: boolean) => void;
  isPanelOpen: boolean;
}

interface ContextInfo {
  context: string;
  namespace: string;
}

const QueryInput: React.FC<QueryInputProps> = ({ 
  onSubmit, 
  isLoading, 
  queryStatus, 
  isHistoryModalOpen, 
  setIsHistoryModalOpen,
  isPanelOpen
}) => {
  const [query, setQuery] = useState('');
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [cursorPosition, setCursorPosition] = useState(0);
  const [selectedSuggestionIndex, setSelectedSuggestionIndex] = useState(-1);
  const [isFocused, setIsFocused] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const [suggestionsPosition, setSuggestionsPosition] = useState({ top: 0, left: 0 });

  const [queryHistory, setQueryHistory] = useState<string[]>([]);

  const [contextInfo, setContextInfo] = useState<ContextInfo | null>(null);

  const [isDryRunMode, setIsDryRunMode] = useState(false);

  useEffect(() => {
    const savedHistory = localStorage.getItem('queryHistory');
    if (savedHistory) {
      setQueryHistory(JSON.parse(savedHistory));
    }
  }, []);

  const saveQueryToHistory = (newQuery: string) => {
    const updatedHistory = [newQuery, ...queryHistory.filter(q => q !== newQuery)].slice(0, 1000);
    setQueryHistory(updatedHistory);
    localStorage.setItem('queryHistory', JSON.stringify(updatedHistory));
  };

  const updateSuggestionsPosition = () => {
    if (textareaRef.current) {
      const cursorPosition = textareaRef.current.selectionEnd;
      const textBeforeCursor = textareaRef.current.value.substring(0, cursorPosition);
      const lines = textBeforeCursor.split('\n');
      const currentLineNumber = lines.length;
      const currentLineText = lines[lines.length - 1];

      const lineHeight = 21; // Adjust this value based on your font size and line height
      const charWidth = 8.4; // Adjust this value based on your font size

      const top = (currentLineNumber * lineHeight) + 15; // 16px for padding
      const left = (currentLineText.length * charWidth) + 6;

      setSuggestionsPosition({ top, left });
    }
  };

  useEffect(() => {
    updateSuggestionsPosition();
  }, [cursorPosition]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const selectedText = window.getSelection()?.toString() || null;
    saveQueryToHistory(query);
    onSubmit(query, selectedText);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && e.shiftKey) {
      e.preventDefault();
      handleSubmit(e);
    } else if (e.key === 'Tab') {
      e.preventDefault();
      if (suggestions.length > 0) {
        insertSuggestion(suggestions[selectedSuggestionIndex !== -1 ? selectedSuggestionIndex : 0]);
      }
    } else if (e.key === 'ArrowDown' && suggestions.length > 0) {
      e.preventDefault();
      setSelectedSuggestionIndex((prevIndex) =>
        prevIndex < suggestions.length - 1 ? prevIndex + 1 : prevIndex
      );
      scrollSuggestionIntoView(selectedSuggestionIndex + 1);
    } else if (e.key === 'ArrowUp' && suggestions.length > 0) {
      e.preventDefault();
      setSelectedSuggestionIndex((prevIndex) => (prevIndex > 0 ? prevIndex - 1 : 0));
      scrollSuggestionIntoView(selectedSuggestionIndex - 1);
    } else if (e.key === 'Enter' && selectedSuggestionIndex !== -1) {
      e.preventDefault();
      insertSuggestion(suggestions[selectedSuggestionIndex]);
    }
  };

  const scrollSuggestionIntoView = (index: number) => {
    if (suggestionsRef.current) {
      const suggestionItems = suggestionsRef.current.getElementsByClassName('suggestion-item');
      if (suggestionItems[index]) {
        suggestionItems[index].scrollIntoView({
          behavior: 'smooth',
          block: 'nearest',
        });
      }
    }
  };

  const insertSuggestion = (suggestion: string) => {
    const newQuery = query.slice(0, cursorPosition) + suggestion + query.slice(cursorPosition);
    const newCursorPosition = cursorPosition + suggestion.length;

    setQuery(newQuery);
    setCursorPosition(newCursorPosition);
    setSuggestions([]);
    setSelectedSuggestionIndex(-1);

    // Update the cursor position in the textarea
    if (textareaRef.current) {
      textareaRef.current.setSelectionRange(newCursorPosition, newCursorPosition);
    }
};

  const debouncedFetchSuggestions = useCallback(
    debounce(async (query: string, position: number) => {
      try {
        const fetchedSuggestions = await fetchAutocompleteSuggestions(query, position);
        // Ensure suggestions are unique
        const uniqueSuggestions = Array.from(new Set(fetchedSuggestions));
        setSuggestions(uniqueSuggestions);
        setSelectedSuggestionIndex(-1);
      } catch (error) {
        console.error('Failed to fetch suggestions:', error);
      }
    }, 300),
    []
  );

  useEffect(() => {
    debouncedFetchSuggestions(query, cursorPosition);
  }, [query, cursorPosition, debouncedFetchSuggestions]);

  const handleQueryChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const newQuery = e.target.value;
    const newPosition = e.target.selectionStart;
    setQuery(newQuery);
    setCursorPosition(newPosition);
  };

  const handleCursorChange = (e: React.SyntheticEvent<HTMLTextAreaElement>) => {
    const newPosition = e.currentTarget.selectionStart;
    setCursorPosition(newPosition);
  };

  const isEndOfLine = () => {
    if (textareaRef.current) {
      const lines = query.split('\n');
      const currentLineIndex = query.substr(0, cursorPosition).split('\n').length - 1;
      const currentLine = lines[currentLineIndex];
      return cursorPosition === query.length || 
             (currentLineIndex < lines.length - 1 && 
              cursorPosition === query.indexOf('\n', query.indexOf(currentLine)) - 1);
    }
    return false;
  };

  const suggestionsRef = useRef<HTMLDivElement>(null);

  const handleSuggestionClick = (e: React.MouseEvent, suggestion: string) => {
    e.preventDefault();
    e.stopPropagation();
    insertSuggestion(suggestion);
    if (textareaRef.current) {
      textareaRef.current.focus();
    }
  };

  const fetchContextInfo = useCallback(async () => {
    try {
      const response = await fetch('/api/context');
      if (!response.ok) {
        throw new Error('Failed to fetch context info');
      }
      const data = await response.json();
      setContextInfo(data);
    } catch (error) {
      console.error('Failed to fetch context info:', error);
    }
  }, []);

  const fetchDryRunState = useCallback(async () => {
    try {
      // Use the new config endpoint
      const response = await fetch('/api/config');
      if (!response.ok) {
        throw new Error('Failed to fetch configuration');
      }
      const data = await response.json();
      setIsDryRunMode(data.dryRun);
    } catch (error) {
      console.error('Failed to fetch configuration:', error);
    }
  }, []);

  useEffect(() => {
    fetchContextInfo();
    fetchDryRunState(); // Fetch the initial configuration
  }, [fetchContextInfo, fetchDryRunState]);

  const toggleDryRunMode = (e: React.MouseEvent) => {
    // Prevent the button from submitting the form
    e.preventDefault();
    e.stopPropagation();
    
    // Use the new config endpoint with the updated value
    fetch('/api/config', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        dryRun: !isDryRunMode,
      }),
    })
      .then(response => response.json())
      .then(data => {
        setIsDryRunMode(data.dryRun);
      })
      .catch(error => {
        console.error('Error updating configuration:', error);
      });
  };

  return (
    <form className={`query-input-form ${isFocused ? 'focused' : ''} ${!isPanelOpen ? 'panel-closed' : ''}`} onSubmit={handleSubmit}>
      <div className="query-editor">
        {contextInfo && (
          <div className="context-indicator">
            ctx: <span className="context">{contextInfo.context}</span>
            {contextInfo.namespace && (
              <>
                ns: <span className="namespace">{contextInfo.namespace}</span>
                <button 
                  type="button"
                  className={`dry-run-toggle ${isDryRunMode ? 'active' : ''}`}
                  onClick={toggleDryRunMode}
                  title="Toggle dry-run mode"
                >
                  {isDryRunMode ? 'Dry Run: ON' : 'Dry Run: OFF'}
                </button>
              </>
            )}
          </div>
        )}
        <SyntaxHighlighter
          language="cypher"
          style={dracula}
          wrapLines={true}
          wrapLongLines={true}
          customStyle={{
            margin: 0,
            padding: '1rem',
            fontSize: '14px',
            lineHeight: '1.5',
            backgroundColor: 'transparent',
            fontFamily: "'Consolas', 'Monaco', 'Andale Mono', 'Ubuntu Mono', monospace",
          }}
        >
          {query}
        </SyntaxHighlighter>
        <textarea
          ref={textareaRef}
          value={query}
          onChange={handleQueryChange}
          onKeyDown={handleKeyDown}
          onSelect={handleCursorChange}
          onFocus={() => setIsFocused(true)}
          onBlur={() => setIsFocused(false)}
          placeholder="Your Cyphernetes query here..."
          rows={5}
          disabled={isLoading}
          className="query-textarea"
          spellCheck="false"
        />
        {isFocused && isEndOfLine() && suggestions.length > 0 && suggestions[0] !== "" && (
          <div 
            ref={suggestionsRef}
            className="suggestions" 
            style={{ 
              top: `${suggestionsPosition.top}px`, 
              left: `${suggestionsPosition.left}px`,
            }}
          >
            {suggestions.map((suggestion, index) => (
              <div
                key={index}
                className={`suggestion-item ${index === selectedSuggestionIndex ? 'highlighted' : ''}`}
                onClick={(e) => handleSuggestionClick(e, suggestion)}
                onMouseDown={(e) => e.preventDefault()} // Prevent blur on click
                style={{
                  padding: '5px 10px',
                  cursor: 'pointer',
                  backgroundColor: 'transparent',
                }}
              >
                {suggestion}
              </div>
            ))}
          </div>
        )}
        <button type="submit" className="submit-button" disabled={isLoading}>
          {isLoading ? 'Executing...' : 'Execute Query'}
        </button>
        <button
          type="button"
          className="history-button"
          onClick={() => setIsHistoryModalOpen(true)}
        >
          History ({navigator.platform.includes('Mac') ? '⌘' : 'Ctrl'}+K)
        </button>
        {queryStatus && (
          <div className="query-status">
            <span className="query-status-count">{queryStatus.numQueries}</span> {queryStatus.numQueries === 1 ? 'query' : 'queries'} 
            <span className={`query-status-result ${queryStatus.status}`}>{queryStatus.status}</span> in 
            <span className="query-status-time">{queryStatus.time.toFixed(2)}s</span>
          </div>
        )}
      </div>
      <HistoryModal
        isOpen={isHistoryModalOpen}
        onClose={() => setIsHistoryModalOpen(false)}
        history={queryHistory}
        onSelectQuery={(selectedQuery) => {
          setQuery(selectedQuery);
          setCursorPosition(selectedQuery.length);
        }}
      />
    </form>
  );
};

// Debounce function
function debounce<F extends (...args: any[]) => any>(func: F, wait: number): (...args: Parameters<F>) => void {
  let timeoutId: ReturnType<typeof setTimeout> | null = null;
  return (...args: Parameters<F>) => {
    if (timeoutId !== null) {
      clearTimeout(timeoutId);
    }
    timeoutId = setTimeout(() => func(...args), wait);
  };
}

export default QueryInput;
