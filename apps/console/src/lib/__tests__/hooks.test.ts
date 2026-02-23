import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useMigrationPolling, useMigrations } from "../hooks";

// ---------------------------------------------------------------------------
// Mock the api module so hooks never make real fetch calls
// ---------------------------------------------------------------------------

vi.mock("../api", () => ({
  listMigrations: vi.fn(),
}));

// Import AFTER vi.mock so we get the mocked version
import { listMigrations } from "../api";
const mockListMigrations = vi.mocked(listMigrations);

const MOCK_DATA = {
  migrations: [
    {
      id: "migration-1",
      name: "First Migration",
      description: "Test migration",
      requiredInputs: [],
      candidates: [],
      steps: [],
      createdAt: new Date().toISOString(),
      cancelledAt: null,
    },
    {
      id: "migration-2",
      name: "Second Migration",
      description: "Another test",
      requiredInputs: [],
      candidates: [],
      steps: [],
      createdAt: new Date().toISOString(),
      cancelledAt: null,
    },
  ],
};

beforeEach(() => {
  mockListMigrations.mockReset();
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// useMigrations
// ---------------------------------------------------------------------------

describe("useMigrations", () => {
  it("starts with loading:true, empty migrations, and no error", () => {
    mockListMigrations.mockResolvedValue(MOCK_DATA);
    const { result } = renderHook(() => useMigrations());

    expect(result.current.loading).toBe(true);
    expect(result.current.migrations).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it("populates migrations and clears loading on success", async () => {
    mockListMigrations.mockResolvedValue(MOCK_DATA);
    const { result } = renderHook(() => useMigrations());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.migrations).toEqual(MOCK_DATA.migrations);
    expect(result.current.error).toBeNull();
  });

  it("sets error and clears loading on failure", async () => {
    mockListMigrations.mockRejectedValue(new Error("network error"));
    const { result } = renderHook(() => useMigrations());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBe("network error");
    expect(result.current.migrations).toEqual([]);
  });

  it("uses fallback message when rejection is not an Error instance", async () => {
    mockListMigrations.mockRejectedValue("plain string");
    const { result } = renderHook(() => useMigrations());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBe("Failed to load");
  });

  it("refetch() re-fetches and updates migrations", async () => {
    mockListMigrations.mockResolvedValueOnce({ migrations: [] });
    const { result } = renderHook(() => useMigrations());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.migrations).toEqual([]);

    mockListMigrations.mockResolvedValueOnce(MOCK_DATA);
    await act(() => result.current.refetch());

    expect(result.current.migrations).toEqual(MOCK_DATA.migrations);
  });

  it("refetch() clears a previous error on success", async () => {
    mockListMigrations.mockRejectedValueOnce(new Error("first failure"));
    const { result } = renderHook(() => useMigrations());

    await waitFor(() => expect(result.current.error).toBe("first failure"));

    mockListMigrations.mockResolvedValueOnce(MOCK_DATA);
    await act(() => result.current.refetch());

    expect(result.current.error).toBeNull();
    expect(result.current.migrations).toEqual(MOCK_DATA.migrations);
  });
});

// ---------------------------------------------------------------------------
// useMigrationPolling
//
// useMigrations and useMigrationPolling share identical refetch logic, so
// state-change behaviour (loading, error, migrations) is covered above.
// These tests focus exclusively on what is unique to useMigrationPolling:
// the interval setup, the polling callback, and cleanup on unmount.
//
// Approach: spy on setInterval/clearInterval rather than vi.useFakeTimers()
// because @testing-library/dom's waitFor also uses setInterval internally —
// faking it deadlocks the test.
// ---------------------------------------------------------------------------

describe("useMigrationPolling", () => {
  it("sets up an interval with the specified duration", () => {
    const spy = vi.spyOn(globalThis, "setInterval");
    mockListMigrations.mockResolvedValue(MOCK_DATA);

    renderHook(() => useMigrationPolling(3000));

    expect(spy).toHaveBeenCalledWith(expect.any(Function), 3000);
  });

  it("calls listMigrations again on each interval tick", () => {
    let tick: (() => void) | null = null;
    vi.spyOn(globalThis, "setInterval").mockImplementation((cb: unknown) => {
      tick = cb as () => void;
      return 99 as unknown as ReturnType<typeof setInterval>;
    });

    mockListMigrations.mockResolvedValue(MOCK_DATA);
    renderHook(() => useMigrationPolling(5000));

    // One call on mount
    expect(mockListMigrations).toHaveBeenCalledTimes(1);

    // Each tick fires another call
    if (tick) (tick as () => void)();
    if (tick) (tick as () => void)();
    expect(mockListMigrations).toHaveBeenCalledTimes(3);
  });

  it("clears the interval on unmount", () => {
    const mockId = 42;
    vi.spyOn(globalThis, "setInterval").mockReturnValue(
      mockId as unknown as ReturnType<typeof setInterval>,
    );
    const clearSpy = vi.spyOn(globalThis, "clearInterval");

    mockListMigrations.mockResolvedValue(MOCK_DATA);
    const { unmount } = renderHook(() => useMigrationPolling(5000));

    unmount();

    expect(clearSpy).toHaveBeenCalledWith(mockId);
  });
});
