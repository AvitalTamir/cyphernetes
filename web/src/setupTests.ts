import '@testing-library/jest-dom';
import 'vitest-canvas-mock';

import { vi } from 'vitest';

// Extend expect with custom matchers
import { expect } from 'vitest';
import * as matchers from '@testing-library/jest-dom/matchers';
expect.extend(matchers);

// Reset all mocks before each test
beforeEach(() => {
  vi.resetAllMocks();
});