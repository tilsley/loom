"use client";

import { useState } from "react";
import { createPortal } from "react-dom";
import { Button, Input } from "@/components/ui";

interface RunInputsModalProps {
  candidateId: string;
  requiredInputs: string[];
  prefilled: Record<string, string>;
  onConfirm: (inputs: Record<string, string>) => void;
  onCancel: () => void;
}

export function RunInputsModal({
  candidateId,
  requiredInputs,
  prefilled,
  onConfirm,
  onCancel,
}: RunInputsModalProps) {
  const [values, setValues] = useState<Record<string, string>>(() => {
    const init: Record<string, string> = {};
    for (const key of requiredInputs) {
      init[key] = prefilled[key] ?? "";
    }
    return init;
  });

  const allFilled = requiredInputs.every((k) => values[k]?.trim());

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!allFilled) return;
    onConfirm(values);
  }

  return createPortal(
    <div
      className="fixed inset-0 z-50 overflow-y-auto bg-black/60 backdrop-blur-sm"
      onClick={onCancel}
    >
      <div className="flex min-h-full items-center justify-center p-4">
        <div
          className="relative w-full max-w-md bg-[var(--color-surface)] border border-zinc-800 rounded-xl shadow-2xl"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="px-5 py-4 border-b border-zinc-800">
            <h3 className="text-sm font-semibold text-zinc-100">Run inputs required</h3>
            <p className="text-xs text-zinc-500 mt-0.5 font-mono truncate">{candidateId}</p>
          </div>

          <form onSubmit={handleSubmit}>
            <div className="px-5 py-4 space-y-4">
              {requiredInputs.map((key) => (
                <div key={key}>
                  <label className="block text-xs font-medium text-zinc-500 uppercase tracking-widest mb-2">
                    {key}
                  </label>
                  <Input
                    type="text"
                    value={values[key] ?? ""}
                    onChange={(e) => setValues((v) => ({ ...v, [key]: e.target.value }))}
                    placeholder={key}
                    className="font-mono"
                    autoFocus={requiredInputs[0] === key}
                  />
                </div>
              ))}
            </div>

            <div className="flex items-center gap-2 px-5 py-4 border-t border-zinc-800">
              <Button type="submit" size="sm" disabled={!allFilled}>
                Queue run
              </Button>
              <button
                type="button"
                onClick={onCancel}
                className="px-3 py-1.5 text-sm text-zinc-500 hover:text-zinc-300 transition-colors"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>,
    document.body,
  );
}
