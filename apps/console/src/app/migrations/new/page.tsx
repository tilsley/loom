"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import {
  registerMigration,
  type RegisterMigrationRequest,
  type StepDefinition,
  type Candidate,
} from "@/lib/api";
import { useRole } from "@/contexts/role-context";
import { ROUTES } from "@/lib/routes";
import { Button, Input, Textarea } from "@/components/ui";

const DEFAULT_STEPS: StepDefinition[] = [
  {
    name: "refactor-api",
    workerApp: "migration-worker",
    config: { targetVersion: "v2" },
  },
];

export default function NewMigration() {
  const router = useRouter();
  const { isAdmin } = useRole();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [candidatesJson, setCandidatesJson] = useState(
    JSON.stringify([{ repo: "acme/billing-api" }, { repo: "acme/user-service" }], null, 2),
  );
  const [stepsJson, setStepsJson] = useState(JSON.stringify(DEFAULT_STEPS, null, 2));
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();

    if (!isAdmin) {
      toast.error("Admin role required to register migrations");
      return;
    }

    let steps: StepDefinition[];
    try {
      steps = JSON.parse(stepsJson);
    } catch {
      toast.error("Invalid JSON in steps field");
      return;
    }

    let candidateList: Candidate[];
    try {
      candidateList = JSON.parse(candidatesJson);
      if (!Array.isArray(candidateList) || !candidateList.every((t) => typeof t.id === "string")) {
        throw new Error("Each candidate must have an id field");
      }
    } catch (err) {
      toast.error(
        err instanceof Error
          ? `Invalid candidates JSON: ${err.message}`
          : "Invalid JSON in targets field",
      );
      return;
    }

    if (!name.trim()) {
      toast.error("Name is required");
      return;
    }
    if (candidateList.length === 0) {
      toast.error("At least one candidate is required");
      return;
    }
    if (!Array.isArray(steps) || steps.length === 0) {
      toast.error("At least one step is required");
      return;
    }

    const req: RegisterMigrationRequest = {
      name: name.trim(),
      description: description.trim() || undefined,
      candidates: candidateList,
      steps,
    };

    setSubmitting(true);
    try {
      const m = await registerMigration(req);
      router.push(ROUTES.migrationDetail(m.id));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to register");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="space-y-8 animate-fade-in-up">
      {/* Breadcrumb */}
      <Link
        href={ROUTES.migrations}
        className="inline-flex items-center gap-1 text-xs text-zinc-500 hover:text-zinc-300 transition-colors"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
          <path
            d="M7 3L4 6l3 3"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        Migrations
      </Link>

      <div>
        <h2 className="text-xl font-semibold tracking-tight text-zinc-50">Register Migration</h2>
        <p className="text-sm text-zinc-500 mt-1">
          Define a reusable migration that can be run multiple times.
        </p>
      </div>

      <form onSubmit={(e) => void handleSubmit(e)} className="space-y-6">
        <div>
          <label className="block text-xs font-medium text-zinc-500 uppercase tracking-widest mb-2">
            Name
          </label>
          <Input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. API v2 migration"
          />
        </div>

        <div>
          <label className="block text-xs font-medium text-zinc-500 uppercase tracking-widest mb-2">
            Description
          </label>
          <Input
            type="text"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Optional description..."
          />
        </div>

        <div>
          <label className="block text-xs font-medium text-zinc-500 uppercase tracking-widest mb-2">
            Candidates
            <span className="ml-2 text-zinc-600 normal-case tracking-normal">JSON array</span>
          </label>
          <Textarea
            value={candidatesJson}
            onChange={(e) => setCandidatesJson(e.target.value)}
            rows={6}
            className="font-mono text-xs"
          />
        </div>

        <div>
          <label className="block text-xs font-medium text-zinc-500 uppercase tracking-widest mb-2">
            Steps
            <span className="ml-2 text-zinc-600 normal-case tracking-normal">JSON array</span>
          </label>
          <Textarea
            value={stepsJson}
            onChange={(e) => setStepsJson(e.target.value)}
            rows={12}
            className="font-mono text-xs"
          />
        </div>

        <div className="flex gap-3 pt-2">
          <Button type="submit" disabled={submitting || !isAdmin}>
            {submitting ? "Registering..." : "Register"}
          </Button>
          <Link
            href={ROUTES.migrations}
            className="px-4 py-2 text-sm text-zinc-500 hover:text-zinc-300 transition-colors"
          >
            Cancel
          </Link>
        </div>
      </form>
    </div>
  );
}
