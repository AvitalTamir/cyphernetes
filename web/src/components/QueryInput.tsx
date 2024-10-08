import React, { useState, useCallback, useRef, useEffect } from 'react';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { dracula } from 'react-syntax-highlighter/dist/esm/styles/prism';
import './QueryInput.css';

interface QueryInputProps {
  onSubmit: (query: string) => void;
  isLoading: boolean;
}

const QueryInput: React.FC<QueryInputProps> = ({ onSubmit, isLoading }) => {
  const [query, setQuery] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const highlighterRef = useRef<HTMLDivElement>(null);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit(query);
  };

  const handleChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setQuery(e.target.value);
  }, []);

  useEffect(() => {
    if (textareaRef.current && highlighterRef.current) {
      highlighterRef.current.scrollTop = textareaRef.current.scrollTop;
      highlighterRef.current.scrollLeft = textareaRef.current.scrollLeft;
    }
  }, [query]);

  return (
    <form className="query-input-form" onSubmit={handleSubmit}>
      <div className="query-editor" ref={highlighterRef}>
        <SyntaxHighlighter
          language="cypher"
          style={dracula}
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
          className="query-textarea"
          value={query}
          onChange={handleChange}
          placeholder="Enter your Cypher query here..."
          spellCheck={false}
        />
      </div>
      <button type="submit" className="submit-button" disabled={isLoading}>
        {isLoading ? 'Executing...' : 'Execute Query'}
      </button>
    </form>
  );
};

export default QueryInput;