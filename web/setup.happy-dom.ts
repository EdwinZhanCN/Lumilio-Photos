import { GlobalRegistrator } from '@happy-dom/global-registrator';
import { afterEach, beforeEach, describe, expect, it, test, vi } from 'vitest';

// Register Happy DOM as the global environment for tests
GlobalRegistrator.register();

// Setup global mocks and test environment
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

// Polyfill any missing browser APIs if needed
// For example:
// global.ResizeObserver = vi.fn().mockImplementation(() => ({
//   observe: vi.fn(),
//   unobserve: vi.fn(),
//   disconnect: vi.fn(),
// }));

// Export vitest functions to be available in test files
export { afterEach, beforeEach, describe, expect, it, test, vi };
