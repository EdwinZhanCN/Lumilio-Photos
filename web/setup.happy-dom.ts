import "@testing-library/jest-dom/vitest";
import { afterEach, vi } from "vitest";

class MemoryStorage implements Storage {
  private store = new Map<string, string>();

  get length() {
    return this.store.size;
  }

  clear() {
    this.store.clear();
  }

  getItem(key: string) {
    return this.store.get(key) ?? null;
  }

  key(index: number) {
    return Array.from(this.store.keys())[index] ?? null;
  }

  removeItem(key: string) {
    this.store.delete(key);
  }

  setItem(key: string, value: string) {
    this.store.set(key, String(value));
  }
}

const testLocalStorage = new MemoryStorage();

Object.defineProperty(globalThis, "localStorage", {
  value: testLocalStorage,
  configurable: true,
});

if (typeof window !== "undefined") {
  Object.defineProperty(window, "localStorage", {
    value: testLocalStorage,
    configurable: true,
  });
}

afterEach(() => {
  testLocalStorage.clear();
  vi.clearAllMocks();
});
