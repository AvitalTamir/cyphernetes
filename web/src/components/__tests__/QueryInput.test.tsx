import React from 'react';
import { render, fireEvent, screen, waitFor, act } from '@testing-library/react';
import { expect, test, describe, vi, beforeEach } from 'vitest';
import QueryInput from '../QueryInput';

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

describe('QueryInput Component', () => {
  beforeEach(() => {
    // Reset all mocks before each test
    vi.clearAllMocks();
    
    // Setup default mock for context API
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ context: 'test-context', namespace: 'test-namespace' })
    });
  });

  test('submits query when button is clicked', () => {
    const mockOnSubmit = vi.fn();
    render(
      <QueryInput
        onSubmit={mockOnSubmit}
        isLoading={false}
        queryStatus={null}
        isHistoryModalOpen={false}
        setIsHistoryModalOpen={() => {}}
        isPanelOpen={true}
      />
    );

    const textarea = screen.getByPlaceholderText('Your Cyphernetes query here...');
    fireEvent.change(textarea, { target: { value: 'MATCH (n) RETURN n' } });

    const submitButton = screen.getByText('Execute Query');
    fireEvent.click(submitButton);

    expect(mockOnSubmit).toHaveBeenCalledWith('MATCH (n) RETURN n', null);
  });

  test('disables submit button when loading', () => {
    render(
      <QueryInput
        onSubmit={() => {}}
        isLoading={true}
        queryStatus={null}
        isHistoryModalOpen={false}
        setIsHistoryModalOpen={() => {}}
        isPanelOpen={true}
      />
    );

    const submitButton = screen.getByText('Executing...');
    expect(submitButton).toBeDisabled();
  });

  test('displays context information when loaded', async () => {
    await act(async () => {
      render(
        <QueryInput
          onSubmit={() => {}}
          isLoading={false}
          queryStatus={null}
          isHistoryModalOpen={false}
          setIsHistoryModalOpen={() => {}}
          isPanelOpen={true}
        />
      );
    });

    // Wait for the context info to be displayed
    await waitFor(() => {
      expect(screen.getByText('test-context')).toBeInTheDocument();
      expect(screen.getByText('test-namespace')).toBeInTheDocument();
    });

    // Verify the fetch was called correctly
    expect(mockFetch).toHaveBeenCalledWith('/api/context');
  });

  test('handles context API error gracefully', async () => {
    // Mock a failed API response
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error'
    });

    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    await act(async () => {
      render(
        <QueryInput
          onSubmit={() => {}}
          isLoading={false}
          queryStatus={null}
          isHistoryModalOpen={false}
          setIsHistoryModalOpen={() => {}}
          isPanelOpen={true}
        />
      );
    });

    // Wait for the error to be logged
    await waitFor(() => {
      expect(consoleSpy).toHaveBeenCalled();
    });

    // Context indicator should not be rendered
    expect(screen.queryByText('test-context')).not.toBeInTheDocument();
    expect(screen.queryByText('test-namespace')).not.toBeInTheDocument();

    consoleSpy.mockRestore();
  });
});