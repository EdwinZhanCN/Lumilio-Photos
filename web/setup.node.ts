import { afterEach, beforeEach, describe, expect, it, test, vi } from 'vitest';

// Setup global mocks and test environment for Node
beforeEach(() => {
  // Add any global mocks or setup here
});

afterEach(() => {
  // Clean up after each test
  vi.clearAllMocks();
  vi.resetAllMocks();
});

// Make vitest functions available globally
global.expect = expect;
global.vi = vi;
global.describe = describe;
global.it = it;
global.test = test;
global.beforeEach = beforeEach;
global.afterEach = afterEach;

// Export vitest functions to be available in test files
export { afterEach, beforeEach, describe, expect, it, test, vi };
