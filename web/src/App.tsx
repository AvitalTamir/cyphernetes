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
  const [originalQueryResult, setOriginalQueryResult] = useState<QueryResponse | null>(null);
  const [filteredResult, setFilteredResult] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isPanelOpen, setIsPanelOpen] = useState(true);
  const [queryStatus, setQueryStatus] = useState<QueryStatus | null>(null);
  const graphRef = useRef<{ resetGraph: () => void } | null>(null);
  const [isHistoryModalOpen, setIsHistoryModalOpen] = useState(false);
  const [aggregateResults, setAggregateResults] = useState<AggregateResult>({});
  const [filterManagedFields, setFilterManagedFields] = useState(true);
  const [darkTheme, setDarkTheme] = useState(false);
  const [format, setFormat] = useState<'yaml' | 'json'>('yaml');
  const [queryTimestamp, setQueryTimestamp] = useState<number>(0);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setIsHistoryModalOpen(prev => !prev);
      }
    };

    window.addEventListener('keydown', handleKeyDown);

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  useEffect(() => {
    if (originalQueryResult && originalQueryResult.result) {
      const resultData = JSON.parse(originalQueryResult.result);
      const filteredData = filterResults(resultData);
      setFilteredResult(JSON.stringify(filteredData, null, 2));
    }
  }, [filterManagedFields, originalQueryResult]);

  const handleQuerySubmit = async (query: string, selectedText: string | null) => {
    setIsLoading(true);
    setError(null);
    setQueryTimestamp(Date.now());
    const startTime = performance.now();
    try {
      if (graphRef.current && typeof graphRef.current.resetGraph === 'function') {
        graphRef.current.resetGraph();
      }

      let textToExecute = selectedText || query;
      textToExecute = textToExecute.replaceAll(/\/\/.*\n/g, '');
      textToExecute = textToExecute.replace(/\n/g, ' ');
      
      const queries = textToExecute
        .replace(/;$/, '')
        .split(';')
        .map(q => q.trim())

      const results: QueryResponse[] = [];
      const uniqueResults = new Set<string>();
      let newAggregateResults: AggregateResult = {};

      for (const singleQuery of queries) {
        const result = await executeQuery(singleQuery);
        if (result && result.result) {
          results.push(result);

          const parsedResult = JSON.parse(result.result);
          for (const [key, value] of Object.entries(parsedResult)) {
            if (key === 'aggregate') {
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

      setOriginalQueryResult(mergedResult);
      const parsedResult = JSON.parse(mergedResult.result);
      const filteredData = filterResults(parsedResult);
      const stringifiedFilteredData = JSON.stringify(filteredData, null, 2);
      setFilteredResult(stringifiedFilteredData);

      const endTime = performance.now();
      setQueryStatus({
        numQueries: queries.length,
        status: 'succeeded',
        time: (endTime - startTime) / 1000,
      });
    } catch (err) {
      console.error('App: Error occurred', err);
      setError('An error occurred while executing the query: ' + err);
      console.error(err);
      setOriginalQueryResult(null);
      setFilteredResult(null);
      if (graphRef.current && typeof graphRef.current.resetGraph === 'function') {
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

  const filterResults = useCallback((results: any) => {
    if (!filterManagedFields) {
      return results;
    }

    const filterObject = (obj: any): any => {
      if (Array.isArray(obj)) {
        return obj.map(filterObject);
      } else if (obj && typeof obj === 'object') {
        const newObj: any = {};
        for (const key in obj) {
          if (key === 'metadata') {
            newObj[key] = { ...obj[key] };
            delete newObj[key].managedFields;
          } else {
            newObj[key] = filterObject(obj[key]);
          }
        }
        return newObj;
      }
      return obj;
    };

    const filtered = filterObject(results);

    return filtered;
  }, [filterManagedFields]);

  const handleNodeHover = useCallback((highlightedNodes: Set<any>) => {
    if (!originalQueryResult || !originalQueryResult.result) {
      return;
    }

    try {
      const resultData = JSON.parse(originalQueryResult.result);
      let filteredData: any;

      if (highlightedNodes.size === 0) {
        filteredData = resultData;
      } else {
        filteredData = {};
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
      }

      const finalFilteredData = filterResults(filteredData);
      setFilteredResult(JSON.stringify(finalFilteredData, null, 2));
    } catch (err) {
      console.error('Error filtering results:', err);
    }
  }, [originalQueryResult, filterResults]);

  const hasResults = originalQueryResult && originalQueryResult.result && Object.keys(JSON.parse(originalQueryResult.result)).length > 0;

  return (
    <div className={`App ${!isPanelOpen ? 'left-sidebar-closed' : ''} ${isHistoryModalOpen ? 'history-modal-open' : ''}`}>
      {!isPanelOpen && (
        <button className="toggle-button" onClick={() => {
          setIsPanelOpen(!isPanelOpen);
          setTimeout(() => {
            window.dispatchEvent(new Event('resize'));
          }, 10);
        }}>
          {"→"}
        </button>
      )}
      <div className={`left-panel ${!isPanelOpen ? 'closed' : ''} ${hasResults ? 'has-results' : ''}`}>
        {isPanelOpen && (
          <>
            <button className="toggle-button panel-close-button" onClick={() => {
              setIsPanelOpen(false);
              setTimeout(() => {
                window.dispatchEvent(new Event('resize'));
              }, 10);
            }}>
              {"←"}
            </button>
            <ResultsDisplay 
              result={filteredResult} 
              error={error}
              darkTheme={darkTheme}
              format={format}
              key={`results-${queryTimestamp}`}
            />
            {hasResults && (
              <div>
              <div className="filter-options-container">
                <label className="custom-checkbox small">
                  <input
                    type="checkbox"
                    checked={filterManagedFields}
                    onChange={(e) => setFilterManagedFields(e.target.checked)}
                  />
                  <span className="checkmark"></span>
                  Filter Managed Fields
                </label>
                <label className="custom-checkbox small">
                  <input
                    type="checkbox"
                    checked={darkTheme}
                    onChange={(e) => setDarkTheme(e.target.checked)}
                  />
                  <span className="checkmark"></span>
                  Flat Background
                </label>
              </div>
              <div className="format-selector">
                <label className="custom-radio">
                  <input
                    type="radio"
                    name="format"
                    value="yaml"
                    checked={format === 'yaml'}
                    onChange={(e) => setFormat('yaml')}
                  />
                  <span className="radio-mark"></span>
                  YAML
                </label>
                <label className="custom-radio">
                  <input
                    type="radio"
                    name="format"
                    value="json"
                    checked={format === 'json'}
                    onChange={(e) => setFormat('json')}
                  />
                  <span className="radio-mark"></span>
                  JSON
                </label>
              </div>
            </div>
            )}
          </>
        )}
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
            isPanelOpen={isPanelOpen}
          />
        </div>
        <div className="graph-visualization">
          <GraphVisualization 
            ref={graphRef}
            data={originalQueryResult?.graph ?? null} 
            onNodeHover={handleNodeHover}
          />
        </div>
      </div>
    </div>
  );
}

export default App;