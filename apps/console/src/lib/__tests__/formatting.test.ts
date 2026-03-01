import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { pluralizeKind, timeAgo } from "../formatting";

describe("timeAgo", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns 'just now' for dates less than 60 seconds ago", () => {
    vi.setSystemTime(new Date("2024-01-01T12:00:30Z"));
    expect(timeAgo("2024-01-01T12:00:00Z")).toBe("just now");
  });

  it("returns minutes for dates 1–59 minutes ago", () => {
    vi.setSystemTime(new Date("2024-01-01T12:05:00Z"));
    expect(timeAgo("2024-01-01T12:00:00Z")).toBe("5m ago");
  });

  it("returns hours for dates 1–23 hours ago", () => {
    vi.setSystemTime(new Date("2024-01-01T14:00:00Z"));
    expect(timeAgo("2024-01-01T12:00:00Z")).toBe("2h ago");
  });

  it("returns days for dates 24+ hours ago", () => {
    vi.setSystemTime(new Date("2024-01-04T12:00:00Z"));
    expect(timeAgo("2024-01-01T12:00:00Z")).toBe("3d ago");
  });
});

describe("pluralizeKind", () => {
  it("appends 's' to the provided kind", () => {
    expect(pluralizeKind("repo")).toBe("repos");
    expect(pluralizeKind("application")).toBe("applications");
  });

  it("defaults to 'candidates' when kind is undefined", () => {
    expect(pluralizeKind(undefined)).toBe("candidates");
  });
});
