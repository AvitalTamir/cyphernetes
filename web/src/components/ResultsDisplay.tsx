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

const ResultsDisplay: React.FC<ResultsDisplayProps> = ({ result, error, darkTheme, format }) => {
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

  const theme = darkTheme ? qtcreatorDark : gradientDark;

  if (error) return <div className="results-display error results-empty">{error}</div>;
  if (!result) return <div className="results-display results-empty">No results yet</div>;

  return (
    <div className="results-display">
      <div className="left-panel-before"></div>
      <div className="results-content">
        <SyntaxHighlighter language={format} style={theme} customStyle={{fontSize: '14px'}} height={'100%'}>
          {formattedResult}
        </SyntaxHighlighter>
      </div>
      <div className="bottom-controls">
        <div className="left-panel-after"></div>
      </div>
    </div>
  );
};

export default ResultsDisplay;