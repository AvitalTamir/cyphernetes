import React, { useState, useEffect } from 'react';
import { Light as SyntaxHighlighter } from 'react-syntax-highlighter';
import json from 'react-syntax-highlighter/dist/esm/languages/hljs/json';
import yaml from 'react-syntax-highlighter/dist/esm/languages/hljs/yaml';
import { gradientDark, qtcreatorDark } from 'react-syntax-highlighter/dist/esm/styles/hljs';
import * as jsYaml from 'js-yaml';
import './ResultsDisplay.css';

SyntaxHighlighter.registerLanguage('json', json);
SyntaxHighlighter.registerLanguage('yaml', yaml);

interface ResultsDisplayProps {
  result: string | null;
  error: string | null;
  darkTheme: boolean;
  format: 'yaml' | 'json';
}

type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };

const ResultsDisplay: React.FC<ResultsDisplayProps> = ({ result, error, darkTheme, format }) => {
  const [originalData, setOriginalData] = useState<JsonValue | null>(null);
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [displayedResult, setDisplayedResult] = useState<string>('');

  // Parse the input data once
  useEffect(() => {
    if (result) {
      try {
        const parsed = JSON.parse(result);
        setOriginalData(parsed);
      } catch (e) {
        console.error('Failed to parse result:', e);
        setOriginalData(null);
      }
    } else {
      setOriginalData(null);
    }
  }, [result]);

  // Filter and format the data whenever search query changes or format changes
  useEffect(() => {
    if (!originalData) {
      setDisplayedResult('');
      return;
    }

    const filterObject = (obj: JsonValue, path: string[] = [], isArrayItem: boolean = false): JsonValue | null => {
      if (typeof obj === 'string' || typeof obj === 'number' || typeof obj === 'boolean') {
        return String(obj).toLowerCase().includes(searchQuery.toLowerCase()) ? obj : null;
      }

      if (Array.isArray(obj)) {
        const filtered = obj
          .map((item, index) => {
            const result = filterObject(item, [...path, String(index)], true);
            // Only include array items that have actual matches (not just a name)
            if (result && typeof result === 'object' && !Array.isArray(result)) {
              const hasOnlyName = Object.keys(result).length === 1 && 'name' in result;
              return hasOnlyName ? null : result;
            }
            return result;
          })
          .filter((item): item is JsonValue => item !== null);
        return filtered.length > 0 ? filtered : null;
      }

      if (obj && typeof obj === 'object') {
        const filtered: { [key: string]: JsonValue } = {};
        let hasMatch = false;

        for (const [key, value] of Object.entries(obj)) {
          // For nested objects, check if the key matches
          if (key.toLowerCase().includes(searchQuery.toLowerCase())) {
            filtered[key] = value;
            hasMatch = true;
          } else {
            const result = filterObject(value, [...path, key], false);
            if (result !== null) {
              // If this is an array item and we found a match, include the name
              if (isArrayItem && !('name' in filtered) && 'name' in obj) {
                filtered['name'] = obj['name'];
              }
              filtered[key] = result;
              hasMatch = true;
            }
          }
        }

        // Clean up empty objects
        if (Object.keys(filtered).length === 0) {
          return null;
        }

        return hasMatch ? filtered : null;
      }

      return null;
    };

    try {
      const filteredData = searchQuery 
        ? filterObject(originalData)
        : originalData;

      const formatted = format === 'yaml'
        ? jsYaml.dump(filteredData)
        : JSON.stringify(filteredData, null, 2);

      setDisplayedResult(formatted);
    } catch (e) {
      console.error('Error formatting result:', e);
      setDisplayedResult('Error formatting result');
    }
  }, [searchQuery, originalData, format]);

  const theme = darkTheme ? qtcreatorDark : gradientDark;

  if (error) return <div className="results-display error results-empty">{error}</div>;
  if (!result) return <div className="results-display results-empty">No results yet</div>;

  return (
    <div className="results-display">
      <div className="search-container">
        <input
          type="text"
          placeholder="Search results..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="results-search-input"
        />
      </div>
      <div className="left-panel-before"></div>
      <div className="results-content">
        <SyntaxHighlighter language={format} style={theme} customStyle={{fontSize: '14px'}} height={'100%'}>
          {displayedResult}
        </SyntaxHighlighter>
      </div>
      <div className="bottom-controls">
        <div className="left-panel-after"></div>
      </div>
    </div>
  );
};

export default ResultsDisplay;