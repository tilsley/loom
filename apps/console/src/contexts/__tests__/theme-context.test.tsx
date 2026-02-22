import { type ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { ThemeProvider, useTheme } from "../theme-context";

const wrapper = ({ children }: { children: ReactNode }) => (
  <ThemeProvider>{children}</ThemeProvider>
);

beforeEach(() => {
  localStorage.clear();
  document.documentElement.classList.remove("light");
});

describe("ThemeProvider / useTheme", () => {
  it("defaults to dark theme", async () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    await waitFor(() => expect(result.current.theme).toBe("dark"));
  });

  it("reads a stored light theme from localStorage on mount", async () => {
    localStorage.setItem("loom-theme", "light");
    const { result } = renderHook(() => useTheme(), { wrapper });
    await waitFor(() => expect(result.current.theme).toBe("light"));
  });

  it("ignores invalid values in localStorage and keeps default", async () => {
    localStorage.setItem("loom-theme", "blue");
    const { result } = renderHook(() => useTheme(), { wrapper });
    await waitFor(() => expect(result.current.theme).toBe("dark"));
  });

  it("toggle switches from dark to light", async () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    await waitFor(() => expect(result.current.theme).toBe("dark"));

    act(() => result.current.toggle());

    expect(result.current.theme).toBe("light");
  });

  it("toggle switches from light to dark", async () => {
    localStorage.setItem("loom-theme", "light");
    const { result } = renderHook(() => useTheme(), { wrapper });
    await waitFor(() => expect(result.current.theme).toBe("light"));

    act(() => result.current.toggle());

    expect(result.current.theme).toBe("dark");
  });

  it("adds the 'light' class to documentElement when theme is light", async () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    await waitFor(() => expect(result.current.theme).toBe("dark"));

    act(() => result.current.toggle());

    expect(document.documentElement.classList.contains("light")).toBe(true);
  });

  it("removes the 'light' class from documentElement when switching to dark", async () => {
    localStorage.setItem("loom-theme", "light");
    const { result } = renderHook(() => useTheme(), { wrapper });
    await waitFor(() => expect(result.current.theme).toBe("light"));

    act(() => result.current.toggle());

    expect(document.documentElement.classList.contains("light")).toBe(false);
  });

  it("persists the changed theme to localStorage", async () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    await waitFor(() => expect(result.current.theme).toBe("dark"));

    act(() => result.current.toggle());

    expect(localStorage.getItem("loom-theme")).toBe("light");
  });
});
