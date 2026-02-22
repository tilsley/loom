import "@testing-library/jest-dom";
import { vi } from "vitest";

// Polyfill localStorage for jsdom environment
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: (key: string) => store[key] || null,
    setItem: (key: string, value: string) => {
      store[key] = value.toString();
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      store = {};
    },
  };
})();

Object.defineProperty(window, "localStorage", {
  value: localStorageMock,
});

// Polyfill vi.stubGlobal if not available
if (!vi.stubGlobal) {
  (vi as any).stubGlobal = (name: string, value: unknown) => {
    Object.defineProperty(window, name, { value });
  };
}

// Polyfill vi.mocked if not available
if (!vi.mocked) {
  (vi as any).mocked = (fn: any) => fn;
}
