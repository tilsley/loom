import { describe, expect, it } from "vitest";
import { buildSearchParams } from "../url";

const params = (qs: string) => new URLSearchParams(qs);

describe("buildSearchParams", () => {
  it("sets a new key and returns a query string with leading '?'", () => {
    expect(buildSearchParams(params(""), "status", "running")).toBe("?status=running");
  });

  it("replaces an existing key", () => {
    expect(buildSearchParams(params("status=running"), "status", "completed")).toBe(
      "?status=completed",
    );
  });

  it("deletes the key when value is null", () => {
    expect(buildSearchParams(params("status=running"), "status", null)).toBe("");
  });

  it("deletes the key when value is 'all'", () => {
    expect(buildSearchParams(params("status=running"), "status", "all")).toBe("");
  });

  it("preserves unrelated params when deleting a key", () => {
    expect(buildSearchParams(params("page=2&status=running"), "status", null)).toBe("?page=2");
  });
});
