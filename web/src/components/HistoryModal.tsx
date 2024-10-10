import React, { useState, useEffect, useRef } from 'react';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { dracula } from 'react-syntax-highlighter/dist/cjs/styles/prism';
import './HistoryModal.css';

interface HistoryModalProps {
  isOpen: boolean;
  onClose: () => void;
  history: string[];
  onSelectQuery: (query: string) => void;
}

const HistoryModal: React.FC<HistoryModalProps> = ({ isOpen, onClose, history, onSelectQuery }) => {
  const [searchTerm, setSearchTerm] = useState('');
  const [filteredHistory, setFilteredHistory] = useState<string[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

  useEffect(() => {
    if (isOpen) {
      setSearchTerm('');
      setSelectedIndex(0);
      searchInputRef.current?.focus();
    }
  }, [isOpen]);

  useEffect(() => {
    const filtered = history.filter(query =>
      query.toLowerCase().includes(searchTerm.toLowerCase())
    );
    setFilteredHistory(filtered);
    setSelectedIndex(0);
  }, [searchTerm, history]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSelectedIndex(prev => (prev < filteredHistory.length - 1 ? prev + 1 : prev));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSelectedIndex(prev => (prev > 0 ? prev - 1 : 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (filteredHistory[selectedIndex]) {
        onSelectQuery(filteredHistory[selectedIndex]);
        onClose();
      }
    } else if (e.key === 'Escape') {
      onClose();
    }
  };

  useEffect(() => {
    if (listRef.current) {
      const selectedElement = listRef.current.children[selectedIndex] as HTMLElement;
      if (selectedElement) {
        selectedElement.scrollIntoView({ block: 'nearest' });
      }
    }
  }, [selectedIndex]);

  if (!isOpen) return null;

  return (
    <div className="history-modal-overlay" onClick={onClose}>
      <div className="history-modal" onClick={e => e.stopPropagation()}>
        <h2>Query History</h2>
        <input
          ref={searchInputRef}
          type="text"
          placeholder="Search history..."
          value={searchTerm}
          onChange={e => setSearchTerm(e.target.value)}
          onKeyDown={handleKeyDown}
          className="history-search-input"
        />
        <ul ref={listRef} className="history-list">
          {filteredHistory.map((query, index) => (
            <li
              key={index}
              className={index === selectedIndex ? 'selected' : ''}
              onClick={() => {
                onSelectQuery(query);
                onClose();
              }}
            >
              <SyntaxHighlighter
                language="cypher"
                style={dracula}
                customStyle={{
                  margin: 0,
                  padding: '8px',
                  fontSize: '14px',
                  lineHeight: '1.5',
                  backgroundColor: 'transparent',
                  fontFamily: "'Consolas', 'Monaco', 'Andale Mono', 'Ubuntu Mono', monospace",
                }}
              >
                {query}
              </SyntaxHighlighter>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
};

export default HistoryModal;