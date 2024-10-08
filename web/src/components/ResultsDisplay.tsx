import React, { useState, useEffect } from 'react';
import { Light as SyntaxHighlighter } from 'react-syntax-highlighter';
import json from 'react-syntax-highlighter/dist/esm/languages/hljs/json';
import yaml from 'react-syntax-highlighter/dist/esm/languages/hljs/yaml';
import { docco } from 'react-syntax-highlighter/dist/esm/styles/hljs';
import * as jsYaml from 'js-yaml';
import './ResultsDisplay.css';

SyntaxHighlighter.registerLanguage('json', json);
SyntaxHighlighter.registerLanguage('yaml', yaml);

interface ResultsDisplayProps {
  result: string | null;
  error: string | null;
}

const ResultsDisplay: React.FC<ResultsDisplayProps> = ({ result, error }) => {
  const [format, setFormat] = useState<'yaml' | 'json'>('yaml');
  const [formattedResult, setFormattedResult] = useState<string>('');

  useEffect(() => {
    if (result) {
      try {
        const jsonData = JSON.parse(result);
        if (format === 'yaml') {
          setFormattedResult(jsYaml.dump(jsonData));
        } else {
          setFormattedResult(JSON.stringify(jsonData, null, 2));
        }
      } catch (e) {
        setFormattedResult(result);
      }
    } else {
      setFormattedResult('');
    }
  }, [result, format]);

  if (error) return <div className="results-display error">{error}</div>;
  if (!result) return <div className="results-display">No results yet</div>;

  return (
    <div className="results-display">
      <div className="results-header">
        <button onClick={() => setFormat('yaml')} className={format === 'yaml' ? 'active' : ''}>YAML</button>
        <button onClick={() => setFormat('json')} className={format === 'json' ? 'active' : ''}>JSON</button>
      </div>
      <SyntaxHighlighter language={format} style={docco} customStyle={{fontSize: '14px'}}>
        {formattedResult}
      </SyntaxHighlighter>
    </div>
  );
};

export default ResultsDisplay;