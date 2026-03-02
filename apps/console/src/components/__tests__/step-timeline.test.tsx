import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, it, vi } from "vitest";
import type { StepState } from "@/lib/api";
import { StepTimeline } from "../step-timeline";

beforeAll(() => {
  // Radix Accordion uses ResizeObserver internally
  global.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
  // Component calls scrollIntoView on the active step
  window.HTMLElement.prototype.scrollIntoView = vi.fn();
});

const candidate = { id: "cand-1", kind: "repo", status: "not_started" as const };

function step(
  stepName: string,
  status: StepState["status"],
  metadata?: Record<string, string>,
): StepState {
  return { stepName, candidate, status, ...(metadata ? { metadata } : {}) };
}

describe("StepTimeline — empty state", () => {
  it("shows a waiting message when there are no results", () => {
    render(<StepTimeline results={[]} />);
    expect(screen.getByText(/Waiting for worker callbacks/)).toBeInTheDocument();
  });
});

describe("StepTimeline — step names and descriptions", () => {
  it("renders every step name", () => {
    render(
      <StepTimeline
        results={[step("clone-repo", "succeeded"), step("run-tests", "in_progress")]}
      />,
    );
    expect(screen.getByText("clone-repo")).toBeInTheDocument();
    expect(screen.getByText("run-tests")).toBeInTheDocument();
  });

  it("renders a step description when provided via stepDescriptions", () => {
    const descriptions = new Map([["clone-repo", "Clone the target repository"]]);
    render(
      <StepTimeline
        results={[step("clone-repo", "in_progress")]}
        stepDescriptions={descriptions}
      />,
    );
    expect(screen.getByText("Clone the target repository")).toBeInTheDocument();
  });

  it("renders nothing for a step not in the descriptions map", () => {
    render(
      <StepTimeline
        results={[step("clone-repo", "in_progress")]}
        stepDescriptions={new Map()}
      />,
    );
    expect(screen.queryByText("Clone the target repository")).toBeNull();
  });
});

describe("StepTimeline — phase labels", () => {
  it.each([
    ["in_progress" as const, "Running"],
    ["failed" as const, "Failed"],
    ["succeeded" as const, "Done"],
    ["merged" as const, "Merged"],
  ])("shows '%s' label for %s status", (status, label) => {
    render(<StepTimeline results={[step("my-step", status)]} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("shows 'Pending' for a plain pending step", () => {
    render(<StepTimeline results={[step("my-step", "pending")]} />);
    expect(screen.getByText("Pending")).toBeInTheDocument();
  });

  it("shows 'PR open' for a pending step with a prUrl", () => {
    render(
      <StepTimeline results={[step("my-step", "pending", { prUrl: "https://github.com/pr/1" })]} />,
    );
    expect(screen.getByText("PR open")).toBeInTheDocument();
  });

  it("shows 'Awaiting review' for a pending step with instructions", () => {
    render(
      <StepTimeline results={[step("my-step", "pending", { instructions: "Do the thing" })]} />,
    );
    expect(screen.getByText("Awaiting review")).toBeInTheDocument();
  });
});

describe("StepTimeline — PR link", () => {
  it("renders a View PR link pointing at the prUrl", () => {
    render(
      <StepTimeline
        results={[step("open-pr", "pending", { prUrl: "https://github.com/pr/42" })]}
      />,
    );
    const link = screen.getByRole("link", { name: /View PR/ });
    expect(link).toHaveAttribute("href", "https://github.com/pr/42");
  });
});

describe("StepTimeline — instructions", () => {
  it("renders instruction text for a manual-review step", () => {
    render(
      <StepTimeline
        results={[step("review", "pending", { instructions: "Check the diff\nApprove if green" })]}
      />,
    );
    expect(screen.getByText("Check the diff")).toBeInTheDocument();
    expect(screen.getByText("Approve if green")).toBeInTheDocument();
  });
});

describe("StepTimeline — retry action", () => {
  it("shows the Retry button for a failed step when onRetry is provided", () => {
    render(<StepTimeline results={[step("deploy", "failed")]} onRetry={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("does not show the Retry button when onRetry is not provided", () => {
    render(<StepTimeline results={[step("deploy", "failed")]} />);
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
  });

  it("calls onRetry with stepName and candidateId when clicked", async () => {
    const onRetry = vi.fn();
    render(<StepTimeline results={[step("deploy", "failed")]} onRetry={onRetry} />);
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetry).toHaveBeenCalledWith("deploy", "cand-1");
  });

  it("disables the Retry button and shows 'Retrying…' after it is clicked", async () => {
    render(<StepTimeline results={[step("deploy", "failed")]} onRetry={vi.fn()} />);
    const btn = screen.getByRole("button", { name: "Retry" });
    await userEvent.click(btn);
    expect(screen.getByRole("button", { name: "Retrying..." })).toBeDisabled();
  });
});

describe("StepTimeline — mark as merged action", () => {
  it("shows Mark as merged for a pending step with a prUrl", () => {
    render(
      <StepTimeline
        results={[step("open-pr", "pending", { prUrl: "https://github.com/pr/1" })]}
        onComplete={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: /Mark as merged/ })).toBeInTheDocument();
  });

  it("calls onComplete with 'merged' when Mark as merged is clicked", async () => {
    const onComplete = vi.fn();
    render(
      <StepTimeline
        results={[step("open-pr", "pending", { prUrl: "https://github.com/pr/1" })]}
        onComplete={onComplete}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /Mark as merged/ }));
    expect(onComplete).toHaveBeenCalledWith("open-pr", "cand-1", "merged");
  });

  it("disables the button and shows 'Sending…' after it is clicked", async () => {
    render(
      <StepTimeline
        results={[step("open-pr", "pending", { prUrl: "https://github.com/pr/1" })]}
        onComplete={vi.fn()}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /Mark as merged/ }));
    expect(screen.getByRole("button", { name: "Sending..." })).toBeDisabled();
  });
});

describe("StepTimeline — review actions", () => {
  it("shows Mark as done and Mark as failed for a pending step with instructions", () => {
    render(
      <StepTimeline
        results={[step("review", "pending", { instructions: "Check it" })]}
        onComplete={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: /Mark as done/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Mark as failed/ })).toBeInTheDocument();
  });

  it("calls onComplete with 'succeeded' when Mark as done is clicked", async () => {
    const onComplete = vi.fn();
    render(
      <StepTimeline
        results={[step("review", "pending", { instructions: "Check it" })]}
        onComplete={onComplete}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /Mark as done/ }));
    expect(onComplete).toHaveBeenCalledWith("review", "cand-1", "succeeded");
  });

  it("calls onComplete with 'failed' when Mark as failed is clicked", async () => {
    const onComplete = vi.fn();
    render(
      <StepTimeline
        results={[step("review", "pending", { instructions: "Check it" })]}
        onComplete={onComplete}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /Mark as failed/ }));
    expect(onComplete).toHaveBeenCalledWith("review", "cand-1", "failed");
  });

  it("disables both buttons and shows 'Sending…' after either is clicked", async () => {
    render(
      <StepTimeline
        results={[step("review", "pending", { instructions: "Check it" })]}
        onComplete={vi.fn()}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /Mark as done/ }));
    expect(screen.getByRole("button", { name: "Sending..." })).toBeDisabled();
    expect(screen.getByRole("button", { name: /Mark as failed/ })).toBeDisabled();
  });
});

describe("StepTimeline — accordion collapsibility", () => {
  it("renders succeeded step triggers as collapsible (cursor-pointer)", () => {
    render(<StepTimeline results={[step("build", "succeeded")]} />);
    // The AccordionTrigger button wraps the step content
    const triggers = screen.getAllByRole("button");
    const stepTrigger = triggers.find((b) => b.textContent?.includes("build"));
    expect(stepTrigger).toHaveClass("cursor-pointer");
  });

  it("renders non-succeeded step triggers as non-interactive (pointer-events-none)", () => {
    render(<StepTimeline results={[step("build", "in_progress")]} />);
    const triggers = screen.getAllByRole("button");
    const stepTrigger = triggers.find((b) => b.textContent?.includes("build"));
    expect(stepTrigger).toHaveClass("pointer-events-none");
  });
});
