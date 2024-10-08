import React, { useState } from 'react';
import './QueryInput.css';

interface QueryInputProps {
  onSubmit: (query: string) => void;
  isLoading: boolean;
}

const QueryInput: React.FC<QueryInputProps> = ({ onSubmit, isLoading }) => {
  const [query, setQuery] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit(query);
  };

  return (
    <form onSubmit={handleSubmit} className="query-input-form">
      <textarea
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Enter your Cyphernetes query here..."
        className="query-textarea"
        disabled={isLoading}
      />
      <button type="submit" className="submit-button" disabled={isLoading}>
        {isLoading ? 'Executing...' : 'Execute Query'}
      </button>
    </form>
  );
};

export default QueryInput;