import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import ResultsDisplay from './ResultsDisplay';

describe('ResultsDisplay', () => {
  const sampleData = {
    d: [
      {
        name: 'test-deployment',
        $: {
          metadata: {
            generation: 3
          },
          spec: {
            replicas: 1,
            template: {
              spec: {
                containers: [
                  {
                    name: 'main',
                    env: [
                      { name: 'API_KEY' },
                      { name: 'DEBUG' }
                    ]
                  }
                ]
              }
            }
          }
        }
      },
      {
        name: 'another-deployment',
        $: {
          spec: {
            replicas: 2
          }
        }
      }
    ]
  };

  it('renders without crashing', () => {
    render(<ResultsDisplay result={null} error={null} darkTheme={false} format="yaml" />);
    expect(screen.getByText('No results yet')).toBeInTheDocument();
  });

  it('renders error message when error is provided', () => {
    render(<ResultsDisplay result={null} error="Test error" darkTheme={false} format="yaml" />);
    expect(screen.getByText('Test error')).toBeInTheDocument();
  });

  it('renders data in YAML format', async () => {
    render(<ResultsDisplay result={JSON.stringify(sampleData)} error={null} darkTheme={false} format="yaml" />);
    await waitFor(() => {
      const content = screen.getByRole('code');
      expect(content.textContent).toContain('test-deployment');
      expect(content.textContent).toContain('replicas: 1');
    });
  });

  it('renders data in JSON format', async () => {
    render(<ResultsDisplay result={JSON.stringify(sampleData)} error={null} darkTheme={false} format="json" />);
    await waitFor(() => {
      const content = screen.getByRole('code');
      expect(content.textContent).toContain('"name": "test-deployment"');
      expect(content.textContent).toContain('"replicas": 1');
    });
  });

  it('filters results when searching', async () => {
    render(<ResultsDisplay result={JSON.stringify(sampleData)} error={null} darkTheme={false} format="yaml" />);
    
    const searchInput = screen.getByPlaceholderText('Search results...');
    fireEvent.change(searchInput, { target: { value: 'replicas' } });

    await waitFor(() => {
      const content = screen.getByRole('code');
      // Should show both deployments with replicas
      expect(content.textContent).toContain('test-deployment');
      expect(content.textContent).toContain('replicas: 1');
      expect(content.textContent).toContain('another-deployment');
      expect(content.textContent).toContain('replicas: 2');
      // Should not show unrelated fields
      expect(content.textContent).not.toContain('API_KEY');
    });
  });

  it('maintains structure when filtering nested objects', async () => {
    render(<ResultsDisplay result={JSON.stringify(sampleData)} error={null} darkTheme={false} format="yaml" />);
    
    const searchInput = screen.getByPlaceholderText('Search results...');
    fireEvent.change(searchInput, { target: { value: 'generation' } });

    await waitFor(() => {
      const content = screen.getByRole('code');
      // Should show the full path to generation
      expect(content.textContent).toContain('test-deployment');
      expect(content.textContent).toContain('metadata');
      expect(content.textContent).toContain('generation: 3');
      // Should not show unrelated fields
      expect(content.textContent).not.toContain('replicas');
      expect(content.textContent).not.toContain('another-deployment');
    });
  });

  it('clears filter when search is empty', async () => {
    render(<ResultsDisplay result={JSON.stringify(sampleData)} error={null} darkTheme={false} format="yaml" />);
    
    const searchInput = screen.getByPlaceholderText('Search results...');
    
    // First filter
    fireEvent.change(searchInput, { target: { value: 'API_KEY' } });
    await waitFor(() => {
      expect(screen.getByRole('code').textContent).not.toContain('another-deployment');
    });

    // Then clear
    fireEvent.change(searchInput, { target: { value: '' } });
    await waitFor(() => {
      const content = screen.getByRole('code');
      // Should show all content again
      expect(content.textContent).toContain('test-deployment');
      expect(content.textContent).toContain('another-deployment');
      expect(content.textContent).toContain('API_KEY');
    });
  });
}); 