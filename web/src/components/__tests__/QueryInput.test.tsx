import React from 'react';
import { render, fireEvent, screen } from '@testing-library/react';
import { expect, test, describe, vi } from 'vitest';
import QueryInput from '../QueryInput';

describe('QueryInput Component', () => {
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
});