import React from 'react';
import { render } from '@testing-library/react';
import { expect, test, describe, beforeEach, vi } from 'vitest';
import GraphVisualization from '../GraphVisualization';

// Mock react-force-graph-2d
vi.mock('react-force-graph-2d', () => ({
  __esModule: true,
  default: React.forwardRef((props: any, ref: any) => <div ref={ref}>Mock ForceGraph2D</div>),
}));

describe('GraphVisualization Component', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  test('renders GraphVisualization component', () => {
    const { getByTestId } = render(
      <GraphVisualization data={null} onNodeHover={() => {}} />
    );
    expect(getByTestId('graph-container')).toBeDefined();
  });
});