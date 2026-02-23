import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { CandidateStatus } from "@/lib/api";
import { ProgressBar } from "../progress-bar";

const c = (...ids: string[]) => ids.map((id) => ({ id }));
const withStatus = (id: string, status: CandidateStatus) => ({ id, status });

describe("ProgressBar", () => {
  it("counts candidates with no associated run as not started", () => {
    render(<ProgressBar candidates={c("a", "b")} />);

    expect(screen.getByText(/not started/)).toBeInTheDocument();
  });

  it("buckets candidates into the correct status categories", () => {
    render(
      <ProgressBar
        candidates={[
          withStatus("a", "completed"),
          withStatus("b", "running"),
          { id: "c" }, // no status → not started
        ]}
      />,
    );

    expect(screen.getByText(/completed/)).toBeInTheDocument();
    expect(screen.getByText(/running/)).toBeInTheDocument();
    expect(screen.getByText(/not started/)).toBeInTheDocument();
  });

  it("treats an unrecognised run status as not started", () => {
    render(
      <ProgressBar
        candidates={[withStatus("a", "cancelled" as CandidateStatus)]}
      />,
    );

    // Falls through to the not_started bucket
    expect(screen.getByText(/not started/)).toBeInTheDocument();
    // "cancelled" never appears as a legend label
    expect(screen.queryByText(/cancelled/)).toBeNull();
  });

  it("only renders legend items for statuses with a non-zero count", () => {
    render(
      <ProgressBar
        candidates={[withStatus("a", "completed")]}
      />,
    );

    expect(screen.getByText(/completed/)).toBeInTheDocument();
    expect(screen.queryByText(/running/)).toBeNull();
    expect(screen.queryByText(/not started/)).toBeNull();
  });

  it("renders bar segments with width proportional to candidate count", () => {
    // 2 of 4 candidates completed → segment should be 50%
    const { container } = render(
      <ProgressBar
        candidates={[
          withStatus("a", "completed"),
          withStatus("b", "completed"),
          { id: "c" },
          { id: "d" },
        ]}
      />,
    );

    expect(container.querySelector('[style*="50%"]')).toBeTruthy();
  });

  it("renders no bar segments and no legend when candidates list is empty", () => {
    render(<ProgressBar candidates={[]} />);

    expect(
      screen.queryByText(/completed|running|not started/),
    ).toBeNull();
  });
});
