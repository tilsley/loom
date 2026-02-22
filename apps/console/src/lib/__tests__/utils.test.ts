import { describe, expect, it } from "vitest";
import { cn } from "../utils";

// cn = twMerge(clsx(inputs)). The tests below verify that both libraries are
// wired together correctly rather than duplicating their own unit test suites.

describe("cn", () => {
  it("filters falsy values", () => {
    expect(cn("foo", false, null, undefined, "bar")).toBe("foo bar");
  });

  it("handles conditional object syntax", () => {
    expect(cn({ foo: true, bar: false, baz: true })).toBe("foo baz");
  });

  it("resolves conflicting Tailwind utilities (tailwind-merge deduplication)", () => {
    // Without twMerge both classes would appear; with it, last wins.
    expect(cn("p-2", "p-4")).toBe("p-4");
    expect(cn("text-red-500", "text-blue-500")).toBe("text-blue-500");
  });
});
