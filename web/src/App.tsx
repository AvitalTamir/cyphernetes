import React, { useState, useCallback, useRef } from 'react';
import QueryInput from './components/QueryInput';
import ResultsDisplay from './components/ResultsDisplay';
import GraphVisualization from './components/GraphVisualization';
import { executeQuery, QueryResponse } from './api/queryApi';
import './App.css'; // We'll create this file for styling

function App() {
  const [queryResult, setQueryResult] = useState<QueryResponse | null>(null);
  const [filteredResult, setFilteredResult] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isPanelOpen, setIsPanelOpen] = useState(true);
  const graphRef = useRef<{ resetGraph: () => void } | null>(null);

  const handleQuerySubmit = async (query: string) => {
    setIsLoading(true);
    setError(null);
    try {
      // Reset the graph before setting new query result
      if (graphRef.current) {
        graphRef.current.resetGraph();
      }
      const result = await executeQuery(query);
      setQueryResult(result);
      setFilteredResult(result.result);
    } catch (err) {
      setError('An error occurred while executing the query: ' + err);
      console.error(err);
      // Clear the graph data and reset the graph visualization
      setQueryResult(null);
      if (graphRef.current) {
        graphRef.current.resetGraph();
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleNodeHover = useCallback((highlightedNodes: Set<any>) => {
    console.log('handleNodeHover called with:', highlightedNodes);
    if (!queryResult || !queryResult.result) {
      console.log('No query result available');
      return;
    }

    try {
      const resultData = JSON.parse(queryResult.result);
      console.log('Parsed result data:', resultData);

      if (highlightedNodes.size === 0) {
        setFilteredResult(JSON.stringify(resultData, null, 2));
      } else {

        const filteredData: any = {};

        for (const [key, value] of Object.entries(resultData)) {
            console.log(`Processing key: ${key}, value:`, value);
            filteredData[key] = [];
            if (Array.isArray(value)) {
            const highlightedNodesArr = Array.from(highlightedNodes)
            highlightedNodesArr.map((highlightedNode) => {
                if (highlightedNode.dataRefId === key) {
                    const includedItems = value.filter((item) => item.name === highlightedNode.name);
                    if (includedItems.length === 0) {
                        return null;
                    } else {
                        filteredData[key] = [...filteredData[key], ...includedItems];
                    }
                }
            });
            console.log(`Filtered ${key}:`, filteredData[key]);
            if (filteredData[key].length === 0) {
                console.log(`Removing empty key: ${key}`);
                delete filteredData[key];
            }
          }
        }

        console.log('Final filtered data:', filteredData);
        setFilteredResult(JSON.stringify(filteredData, null, 2));
      }
    } catch (err) {
      console.error('Error filtering results:', err);
    }
  }, [queryResult]);

  return (
    <div className={`App ${!isPanelOpen ? 'left-sidebar-closed' : ''}`}>
      <button className="toggle-button" onClick={() => {
        setIsPanelOpen(!isPanelOpen);
        setTimeout(() => {
          window.dispatchEvent(new Event('resize'));
        }, 10);
      }}>
        {isPanelOpen ? '×' : '→'}
      </button>
      <div className={`left-panel ${!isPanelOpen ? 'closed' : ''}`}>
        {isPanelOpen && <ResultsDisplay result={filteredResult} error={error} />}
      </div>
      <div className="right-panel">
        <div className="query-input">
          <QueryInput onSubmit={handleQuerySubmit} isLoading={isLoading} />
        </div>
        <div className="graph-visualization">
          <GraphVisualization 
            ref={graphRef}
            data={queryResult?.graph ?? null} 
            onNodeHover={handleNodeHover}
          />
        </div>
      </div>
    </div>
  );
}

export default App;