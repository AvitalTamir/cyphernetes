import React, { useState } from 'react';
import QueryInput from './components/QueryInput';
import ResultsDisplay from './components/ResultsDisplay';
import GraphVisualization from './components/GraphVisualization';
import { executeQuery, QueryResponse } from './api/queryApi';
import './App.css'; // We'll create this file for styling

function App() {
  const [queryResult, setQueryResult] = useState<QueryResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleQuerySubmit = async (query: string) => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await executeQuery(query);
      setQueryResult(result);
    } catch (err) {
      setError('An error occurred while executing the query.');
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="App">
      <div className="left-panel">
        <ResultsDisplay result={queryResult?.result ?? null} error={error} />
      </div>
      <div className="right-panel">
        <div className="query-input">
          <QueryInput onSubmit={handleQuerySubmit} isLoading={isLoading} />
        </div>
        <div className="graph-visualization">
          <GraphVisualization data={queryResult?.graph ?? null} />
        </div>
      </div>
    </div>
  );
}

export default App;