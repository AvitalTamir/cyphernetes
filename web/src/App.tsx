import React, { useState, useCallback, useRef, useEffect } from 'react';
import QueryInput from './components/QueryInput';
import ResultsDisplay from './components/ResultsDisplay';
import GraphVisualization from './components/GraphVisualization';
import { executeQuery, QueryResponse } from './api/queryApi';
import './App.css';

interface AccumulatedResult {
  [key: string]: any[];
}

interface QueryStatus {
  numQueries: number;
  status: 'succeeded' | 'failed';
  time: number;
}

interface AggregateResult {
  [key: string]: any;
}

function App() {
  const [queryResult, setQueryResult] = useState<QueryResponse | null>(null);
  const [filteredResult, setFilteredResult] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isPanelOpen, setIsPanelOpen] = useState(true);
  const [queryStatus, setQueryStatus] = useState<QueryStatus | null>(null);
  const graphRef = useRef<{ resetGraph: () => void } | null>(null);
  const [isHistoryModalOpen, setIsHistoryModalOpen] = useState(false);
  const [aggregateResults, setAggregateResults] = useState<AggregateResult>({});

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'h') {
        e.preventDefault();
        setIsHistoryModalOpen(prev => !prev);
      }
    };

    window.addEventListener('keydown', handleKeyDown);

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  const handleQuerySubmit = async (query: string, selectedText: string | null) => {
    setIsLoading(true);
    setError(null);
    const startTime = performance.now();
    try {
      if (graphRef.current) {
        graphRef.current.resetGraph();
      }

      let textToExecute = selectedText || query;
      textToExecute = textToExecute.replace(/\n/g, ' ');
      const queries = textToExecute
        .split(';')
        .map(q => q.trim())
        .filter(q => q && !q.startsWith('//'));

      const results: QueryResponse[] = [];
      const uniqueResults = new Set<string>();
      let newAggregateResults: AggregateResult = {};

      for (const singleQuery of queries) {
        const result = await executeQuery(singleQuery);
        results.push(result);

        if (result.result) {
          const parsedResult = JSON.parse(result.result);
          for (const [key, value] of Object.entries(parsedResult)) {
            if (key === 'aggregate') {
              // Merge aggregate results
              if (typeof value === 'object' && value !== null) {
                newAggregateResults = { ...newAggregateResults, ...value };
              } else {
                console.warn(`Unexpected aggregate value type: ${typeof value}`);
              }
            } else if (Array.isArray(value)) {
              for (const item of value) {
                uniqueResults.add(JSON.stringify({ [key]: item }));
              }
            }
          }
        }
      }

      setAggregateResults(newAggregateResults);

      // Merge results
      const mergedResult: QueryResponse = {
        result: JSON.stringify(
          {
            ...Array.from(uniqueResults).reduce((acc: AccumulatedResult, curr) => {
              const parsed = JSON.parse(curr);
              const key = Object.keys(parsed)[0];
              if (!acc[key]) acc[key] = [];
              acc[key].push(parsed[key]);
              return acc;
            }, {}),
            ...(Object.keys(newAggregateResults).length > 0 ? { aggregate: newAggregateResults } : {})
          },
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

      const endTime = performance.now();
      setQueryStatus({
        numQueries: queries.length,
        status: 'succeeded',
        time: (endTime - startTime) / 1000,
      });
    } catch (err) {
      setError('An error occurred while executing the query: ' + err);
      console.error(err);
      setQueryResult(null);
      if (graphRef.current) {
        graphRef.current.resetGraph();
      }

      const endTime = performance.now();
      setQueryStatus({
        numQueries: 1,
        status: 'failed',
        time: (endTime - startTime) / 1000,
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleNodeHover = useCallback((highlightedNodes: Set<any>) => {
    if (!queryResult || !queryResult.result) {
      return;
    }

    try {
      const resultData = JSON.parse(queryResult.result);

      if (highlightedNodes.size === 0) {
        // Show all results, including aggregate, when no nodes are highlighted
        setFilteredResult(JSON.stringify(resultData, null, 2));
      } else {
        const filteredData: any = {};

        for (const [key, value] of Object.entries(resultData)) {
          if (key !== 'aggregate' && Array.isArray(value)) {
            filteredData[key] = [];
            const highlightedNodesArr = Array.from(highlightedNodes);
            highlightedNodesArr.forEach((highlightedNode) => {
              if (highlightedNode.dataRefId === key) {
                const includedItems = value.filter((item) => item.name === highlightedNode.name);
                if (includedItems.length > 0) {
                  filteredData[key] = [...filteredData[key], ...includedItems];
                }
              }
            });
            if (filteredData[key].length === 0) {
              delete filteredData[key];
            }
          }
        }

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
        {"×"}
      </button>
      <div className={`left-panel ${!isPanelOpen ? 'closed' : ''}`}>
        {isPanelOpen && <ResultsDisplay result={filteredResult} error={error} />}
      </div>
      <div className="right-panel">
        <button className="toggle-button right-toggle" onClick={() => {
            setIsPanelOpen(!isPanelOpen);
            setTimeout(() => {
            window.dispatchEvent(new Event('resize'));
            }, 10);
        }}>
            {"→"}
        </button>
        <div className="query-input">
          <QueryInput
            onSubmit={handleQuerySubmit}
            isLoading={isLoading}
            queryStatus={queryStatus}
            isHistoryModalOpen={isHistoryModalOpen}
            setIsHistoryModalOpen={setIsHistoryModalOpen}
          />
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