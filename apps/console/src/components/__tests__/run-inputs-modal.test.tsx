import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { RunInputsModal } from "../run-inputs-modal";

const baseProps = {
  candidateId: "org/repo@main",
  requiredInputs: ["token", "env"],
  prefilled: {},
  onConfirm: vi.fn(),
  onCancel: vi.fn(),
};

beforeEach(() => {
  baseProps.onConfirm.mockReset();
  baseProps.onCancel.mockReset();
});

describe("RunInputsModal", () => {
  it("renders an input field for each required input", () => {
    render(<RunInputsModal {...baseProps} />);

    expect(screen.getByPlaceholderText("token")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("env")).toBeInTheDocument();
  });

  it("pre-fills inputs from the prefilled prop", () => {
    render(
      <RunInputsModal
        {...baseProps}
        prefilled={{ token: "mytoken", env: "prod" }}
      />,
    );

    expect(screen.getByPlaceholderText("token")).toHaveValue("mytoken");
    expect(screen.getByPlaceholderText("env")).toHaveValue("prod");
  });

  it("disables the submit button when any input is empty", () => {
    render(<RunInputsModal {...baseProps} />);

    expect(screen.getByRole("button", { name: "Queue run" })).toBeDisabled();
  });

  it("disables the submit button when inputs are whitespace-only", async () => {
    const user = userEvent.setup();
    render(<RunInputsModal {...baseProps} />);

    await user.type(screen.getByPlaceholderText("token"), "   ");
    await user.type(screen.getByPlaceholderText("env"), "   ");

    expect(screen.getByRole("button", { name: "Queue run" })).toBeDisabled();
  });

  it("enables the submit button once all inputs are filled", async () => {
    const user = userEvent.setup();
    render(<RunInputsModal {...baseProps} />);

    await user.type(screen.getByPlaceholderText("token"), "abc");
    await user.type(screen.getByPlaceholderText("env"), "prod");

    expect(
      screen.getByRole("button", { name: "Queue run" }),
    ).not.toBeDisabled();
  });

  it("calls onConfirm with the current input values on submit", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(<RunInputsModal {...baseProps} onConfirm={onConfirm} />);

    await user.type(screen.getByPlaceholderText("token"), "mytoken");
    await user.type(screen.getByPlaceholderText("env"), "production");
    await user.click(screen.getByRole("button", { name: "Queue run" }));

    expect(onConfirm).toHaveBeenCalledWith({
      token: "mytoken",
      env: "production",
    });
  });

  it("calls onCancel when the Cancel button is clicked", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    render(<RunInputsModal {...baseProps} onCancel={onCancel} />);

    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("calls onCancel when the backdrop is clicked", () => {
    const onCancel = vi.fn();
    render(<RunInputsModal {...baseProps} onCancel={onCancel} />);

    // The portal renders directly to document.body; the backdrop is the
    // outermost fixed overlay div with its own onClick={onCancel}.
    const backdrop = document.body.querySelector(".fixed.inset-0");
    if (backdrop) {
      fireEvent.click(backdrop);
    }

    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("does not call onCancel when clicking inside the dialog", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    render(<RunInputsModal {...baseProps} onCancel={onCancel} />);

    // The inner dialog div calls e.stopPropagation(), so clicks inside
    // should never reach the backdrop's onCancel handler.
    await user.click(screen.getByText("Run inputs required"));

    expect(onCancel).not.toHaveBeenCalled();
  });
});
