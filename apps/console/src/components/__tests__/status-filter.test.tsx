import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { StatusFilter } from "../status-filter";

const counts = { running: 3, completed: 5, not_started: 2 };

describe("StatusFilter", () => {
  it("renders all four filter options", () => {
    render(<StatusFilter counts={counts} active="all" onChange={() => {}} />);
    expect(screen.getByText("All")).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(screen.getByText("Completed")).toBeInTheDocument();
    expect(screen.getByText("Not started")).toBeInTheDocument();
  });

  it("shows the total count next to All", () => {
    // 3 + 5 + 2 = 10
    render(<StatusFilter counts={counts} active="all" onChange={() => {}} />);
    expect(screen.getByText("10")).toBeInTheDocument();
  });

  it("shows individual counts next to each status option", () => {
    render(<StatusFilter counts={counts} active="all" onChange={() => {}} />);
    expect(screen.getByText("3")).toBeInTheDocument(); // running
    expect(screen.getByText("5")).toBeInTheDocument(); // completed
    expect(screen.getByText("2")).toBeInTheDocument(); // not_started
  });

  it("shows 0 for statuses absent from the counts object", () => {
    render(<StatusFilter counts={{ running: 1 }} active="all" onChange={() => {}} />);
    const zeros = screen.getAllByText("0");
    // completed and not_started are missing → both show 0
    expect(zeros).toHaveLength(2);
  });

  it("calls onChange with the key of the clicked filter", async () => {
    const onChange = vi.fn();
    render(<StatusFilter counts={counts} active="all" onChange={onChange} />);
    await userEvent.click(screen.getByText("Running"));
    expect(onChange).toHaveBeenCalledWith("running");
  });

  it("does not call onChange when clicking the already-active filter", async () => {
    const onChange = vi.fn();
    render(<StatusFilter counts={counts} active="running" onChange={onChange} />);
    await userEvent.click(screen.getByText("Running"));
    // Radix fires onValueChange("") on deselect; the guard `if (v)` prevents forwarding it
    expect(onChange).not.toHaveBeenCalled();
  });
});
