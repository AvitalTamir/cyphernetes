import React from 'react';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { expect, test, describe, vi, beforeEach } from 'vitest';
import { executeQuery } from '../api/queryApi';
import App from '../App';

// Mock the entire queryApi module
vi.mock('../api/queryApi', () => ({
  executeQuery: vi.fn(),
  fetchAutocompleteSuggestions: vi.fn().mockResolvedValue([]),
  convertResourceName: vi.fn().mockResolvedValue(''),
}));

// Mock child components
vi.mock('../components/QueryInput', () => ({
  default: ({ onSubmit }: any) => (
    <button onClick={() => {
      console.log('QueryInput: onSubmit called');
      onSubmit('MATCH (n) RETURN n');
    }} data-testid="mock-query-input">
      Mock QueryInput
    </button>
  ),
}));

vi.mock('../components/ResultsDisplay', () => ({
  default: ({ result }: any) => {
    console.log('ResultsDisplay: Rendering with result:', result);
    return <div data-testid="results-display">{result}</div>;
  },
}));

vi.mock('../components/GraphVisualization', () => {
  return {
    default: React.forwardRef((props: any, ref: any) => {
      React.useImperativeHandle(ref, () => ({
        resetGraph: vi.fn(),
      }));
      return <div data-testid="mock-graph-visualization">Mock GraphVisualization</div>;
    }),
  };
});

describe('App Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    console.log('Test: beforeEach called');
  });

  test('renders child components', () => {
    console.log('Test: Rendering App component');
    render(<App />);

    expect(screen.getByText('Mock QueryInput')).toBeDefined();
    expect(screen.getByTestId('results-display')).toBeDefined();
    expect(screen.getByTestId('mock-graph-visualization')).toBeDefined();
    console.log('Test: Child components rendered successfully');
  });

  test('submits query and updates results', async () => {
    console.log('Test: Starting submit query test');
    const mockQueryResult = {
      result: JSON.stringify({ data: [{ name: "test result", metadata: { managedFields: {} } }] }),
      graph: JSON.stringify({ Nodes: [], Edges: [] }),
    };
    (executeQuery as jest.Mock).mockResolvedValue(mockQueryResult);
    console.log('Test: Mock query result set up');

    console.log('Test: Rendering App component');
    render(<App />);

    const queryInput = screen.getByTestId('mock-query-input');
    console.log('Test: Found query input');
    
    await act(async () => {
      console.log('Test: Clicking query input');
      fireEvent.click(queryInput);
    });

    console.log('Test: Waiting for executeQuery to be called');
    await waitFor(() => {
      expect(executeQuery).toHaveBeenCalledWith('MATCH (n) RETURN n');
    });
    console.log('Test: executeQuery was called');

    console.log('Test: Waiting for results to update');
    await waitFor(() => {
      const resultsDisplay = screen.getByTestId('results-display');
      console.log('Test: Current results display content:', resultsDisplay.textContent);
      expect(resultsDisplay.textContent).toContain('"name": "test result"');
    }, { timeout: 10000 });
    console.log('Test: Results updated successfully');
  }, 15000);

  test('filters managed fields when checkbox is checked', async () => {
    console.log('Test: Starting filter managed fields test');
    const mockQueryResult = {
      result: JSON.stringify({
        data: [
          {
            name: "test result",
            metadata: {
              managedFields: { some: "data" },
              otherField: "should remain"
            },
            nestedObject: {
              metadata: {
                managedFields: { some: "nested data" },
                otherNestedField: "should also remain"
              }
            }
          }
        ]
      }),
      graph: JSON.stringify({ Nodes: [], Edges: [] }),
    };
    (executeQuery as jest.Mock).mockResolvedValue(mockQueryResult);

    render(<App />);

    // Submit a query
    const queryInput = screen.getByTestId('mock-query-input');
    await act(async () => {
      fireEvent.click(queryInput);
    });

    // Wait for results to appear and check if managedFields are filtered
    await waitFor(() => {
      const resultsDisplay = screen.getByTestId('results-display');
      expect(resultsDisplay.textContent).toContain('"otherField": "should remain"');
      expect(resultsDisplay.textContent).toContain('"otherNestedField": "should also remain"');
      expect(resultsDisplay.textContent).not.toContain('"managedFields"');
    }, { timeout: 3000 });

    console.log('Test: Filter managed fields test completed');
  });
});