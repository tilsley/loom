import { type ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { RoleProvider, useRole } from "../role-context";

const wrapper = ({ children }: { children: ReactNode }) => (
  <RoleProvider>{children}</RoleProvider>
);

beforeEach(() => {
  localStorage.clear();
});

describe("RoleProvider / useRole", () => {
  it("defaults to operator after hydration", async () => {
    const { result } = renderHook(() => useRole(), { wrapper });
    await waitFor(() => expect(result.current.role).toBe("operator"));
  });

  it("reads admin role from localStorage on mount", async () => {
    localStorage.setItem("loom-role", "admin");
    const { result } = renderHook(() => useRole(), { wrapper });
    await waitFor(() => expect(result.current.role).toBe("admin"));
  });

  it("ignores invalid values in localStorage and keeps default", async () => {
    localStorage.setItem("loom-role", "superadmin");
    const { result } = renderHook(() => useRole(), { wrapper });
    await waitFor(() => expect(result.current.role).toBe("operator"));
  });

  it("setRole updates the role", async () => {
    const { result } = renderHook(() => useRole(), { wrapper });
    await waitFor(() => expect(result.current.role).toBe("operator"));

    act(() => result.current.setRole("admin"));

    expect(result.current.role).toBe("admin");
  });

  it("setRole persists the new role to localStorage", async () => {
    const { result } = renderHook(() => useRole(), { wrapper });
    await waitFor(() => expect(result.current.role).toBe("operator"));

    act(() => result.current.setRole("admin"));

    expect(localStorage.getItem("loom-role")).toBe("admin");
  });

  it("isAdmin is true when role is admin", async () => {
    localStorage.setItem("loom-role", "admin");
    const { result } = renderHook(() => useRole(), { wrapper });
    await waitFor(() => expect(result.current.isAdmin).toBe(true));
  });

  it("isAdmin is false when role is operator", async () => {
    const { result } = renderHook(() => useRole(), { wrapper });
    await waitFor(() => expect(result.current.isAdmin).toBe(false));
  });
});
