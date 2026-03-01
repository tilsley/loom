import { describe, expect, it } from "vitest";
import type { Candidate } from "@/lib/api";
import type { components } from "@/lib/api.gen";
import { mergeInputsIntoCandidate, prefillInputs, prefillInputsFromUrl } from "../inputs";

type InputDefinition = components["schemas"]["InputDefinition"];

const input = (name: string): InputDefinition => ({ name, label: name });

const candidate = (metadata?: Record<string, string>): Candidate => ({
  id: "c",
  kind: "repo",
  ...(metadata ? { metadata } : {}),
});

describe("prefillInputs", () => {
  it("uses candidate metadata as default values", () => {
    const result = prefillInputs([input("repo"), input("env")], candidate({ repo: "billing", env: "prod" }));
    expect(result).toEqual({ repo: "billing", env: "prod" });
  });

  it("falls back to empty string when metadata key is missing", () => {
    const result = prefillInputs([input("repo")], candidate({ env: "prod" }));
    expect(result).toEqual({ repo: "" });
  });

  it("returns empty object when required inputs list is empty", () => {
    expect(prefillInputs([], candidate({ repo: "billing" }))).toEqual({});
  });
});

describe("prefillInputsFromUrl", () => {
  const mockParams = (entries: Record<string, string>) => ({
    get: (name: string) => entries[name] ?? null,
  });

  it("prefers URL params over candidate metadata", () => {
    const { values } = prefillInputsFromUrl(
      [input("repo")],
      candidate({ repo: "from-metadata" }),
      mockParams({ repo: "from-url" }),
    );
    expect(values.repo).toBe("from-url");
  });

  it("falls back to metadata when URL param is absent", () => {
    const { values } = prefillInputsFromUrl(
      [input("repo")],
      candidate({ repo: "from-metadata" }),
      mockParams({}),
    );
    expect(values.repo).toBe("from-metadata");
  });

  it("sets allFromUrl=true only when every input has a URL param", () => {
    const inputs = [input("repo"), input("env")];
    const { allFromUrl: yes } = prefillInputsFromUrl(inputs, candidate(), mockParams({ repo: "r", env: "e" }));
    const { allFromUrl: no } = prefillInputsFromUrl(inputs, candidate(), mockParams({ repo: "r" }));
    expect(yes).toBe(true);
    expect(no).toBe(false);
  });
});

describe("mergeInputsIntoCandidate", () => {
  it("returns candidate unchanged when required inputs is empty", () => {
    const c = candidate({ existing: "value" });
    expect(mergeInputsIntoCandidate(c, [], { extra: "x" })).toBe(c);
  });

  it("merges inputs into candidate metadata, inputs winning on conflict", () => {
    const c = candidate({ repo: "old", team: "platform" });
    const result = mergeInputsIntoCandidate(c, [input("repo")], { repo: "new" });
    expect(result.metadata).toEqual({ repo: "new", team: "platform" });
  });

  it("does not mutate the original candidate", () => {
    const c = candidate({ repo: "old" });
    mergeInputsIntoCandidate(c, [input("repo")], { repo: "new" });
    expect(c.metadata?.repo).toBe("old");
  });
});
