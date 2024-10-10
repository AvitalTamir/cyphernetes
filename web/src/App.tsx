import React, { useState, useCallback, useRef } from 'react';
import QueryInput from './components/QueryInput';
import ResultsDisplay from './components/ResultsDisplay';
import GraphVisualization from './components/GraphVisualization';
import { executeQuery, QueryResponse } from './api/queryApi';
import './App.css'; // We'll create this file for styling

interface AccumulatedResult {
  [key: string]: any[];
}

function App() {
  const [queryResult, setQueryResult] = useState<QueryResponse | null>(null);
  const [filteredResult, setFilteredResult] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isPanelOpen, setIsPanelOpen] = useState(true);
  const graphRef = useRef<{ resetGraph: () => void } | null>(null);

  const handleQuerySubmit = async (query: string, selectedText: string | null) => {
    setIsLoading(true);
    setError(null);
    try {
      if (graphRef.current) {
        graphRef.current.resetGraph();
      }

      const textToExecute = selectedText || query;
      const queries = textToExecute
        .split(';')
        .map(q => q.trim())
        .filter(q => q && !q.startsWith('//'));

      const results: QueryResponse[] = [];
      const uniqueResults = new Set<string>();

      for (const singleQuery of queries) {
        const result = await executeQuery(singleQuery);
        results.push(result);

        // Prevent duplicates when merging results
        if (result.result) {
          const parsedResult = JSON.parse(result.result);
          for (const [key, value] of Object.entries(parsedResult)) {
            if (Array.isArray(value)) {
              for (const item of value) {
                uniqueResults.add(JSON.stringify({ [key]: item }));
              }
            }
          }
        }
      }

      // Merge results
      const mergedResult: QueryResponse = {
        result: JSON.stringify(
          Array.from(uniqueResults).reduce((acc: AccumulatedResult, curr) => {
            const parsed = JSON.parse(curr);
            const key = Object.keys(parsed)[0];
            if (!acc[key]) acc[key] = [];
            acc[key].push(parsed[key]);
            return acc;
          }, {}),
          null,
          2
        ),
        graph: results.reduce((acc, curr) => {
          if (curr.graph) {
            const accJson = acc ? JSON.parse(acc) : { Nodes: [], Edges: [] };
            const currGraphJson = JSON.parse(curr.graph);
            const currNodes = currGraphJson.Nodes ?? [];
            const currEdges = currGraphJson.Edges ?? [];
            const accNodes = accJson.Nodes;
            const accEdges = accJson.Edges;
            return JSON.stringify({
              Nodes: [...accNodes, ...currNodes],
              Edges: [...accEdges, ...currEdges]
            });
          }
          return acc;
        }, ''),
      };
      setQueryResult(mergedResult);
      setFilteredResult(mergedResult.result);
    } catch (err) {
      setError('An error occurred while executing the query: ' + err);
      console.error(err);
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
            // @ts-ignore
            data={queryResult?.graph ?? null} 
            onNodeHover={handleNodeHover}
          />
        </div>
      </div>
    </div>
  );
}

export default App;