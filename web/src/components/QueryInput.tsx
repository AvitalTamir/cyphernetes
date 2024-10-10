import React, { useState, useRef, KeyboardEvent } from 'react';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { dracula } from 'react-syntax-highlighter/dist/cjs/styles/prism';
import './QueryInput.css';

interface QueryInputProps {
  onSubmit: (query: string, selectedText: string | null) => void;
  isLoading: boolean;
}

const QueryInput: React.FC<QueryInputProps> = ({ onSubmit, isLoading }) => {
  const [query, setQuery] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const selectedText = window.getSelection()?.toString() || null;
    onSubmit(query, selectedText);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && e.shiftKey) {
      e.preventDefault();
      handleSubmit(e);
    }
  };

  return (
    <form className="query-input-form" onSubmit={handleSubmit}>
         <div className="query-editor">
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
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="Your Cyphernetes query here..."
        rows={5}
        disabled={isLoading}
        className="query-textarea"
        spellCheck="false"

      />
      <button type="submit" className="submit-button" disabled={isLoading}>
        {isLoading ? 'Executing...' : 'Execute Query'}
      </button>
    </div>
    </form>
  );
};

export default QueryInput;