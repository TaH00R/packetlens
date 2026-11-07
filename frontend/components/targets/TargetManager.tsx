"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { parseTarget, parseTargets, targetRequest } from "@/lib/targets";
import type { MonitoringTarget, TargetInput } from "@/types/targets";

const empty: TargetInput = {
  type: "tcp",
  address: "",
  enabled: true,
  interval_ms: 5000,
  timeout_ms: 3000,
};
const fieldClass =
  "w-full rounded-lg border border-white/10 bg-[#0B1016] px-3 py-2 text-sm text-gray-200";
const buttonClass =
  "rounded-lg border border-teal-500/30 px-3 py-2 text-sm text-teal-300 disabled:opacity-40";

export default function TargetManager() {
  const [tokenInput, setTokenInput] = useState("");
  const [token, setToken] = useState("");
  const [targets, setTargets] = useState<MonitoringTarget[] | null>(null);
  const [form, setForm] = useState<TargetInput>(empty);
  const [editing, setEditing] = useState<string | null>(null);
  const [removing, setRemoving] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const controller = useRef<AbortController | null>(null);
  useEffect(
    () => () => {
      controller.current?.abort();
      controller.current = null;
      // Next can hide a route while retaining its state. Re-lock on cleanup so
      // returning to a cached page cannot restore its credential or pending UI.
      setToken("");
      setTokenInput("");
      setTargets(null);
      setBusy(false);
      setEditing(null);
      setRemoving(null);
      setForm(empty);
      setError(null);
      setMessage(null);
    },
    [],
  );

  async function perform(action: (signal: AbortSignal) => Promise<void>) {
    controller.current?.abort();
    const current = new AbortController();
    controller.current = current;
    const timer = window.setTimeout(() => current.abort(), 10_000);
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      await action(current.signal);
    } catch (cause) {
      if (controller.current === current)
        setError(
          current.signal.aborted
            ? "Request timed out. Refresh the target list before retrying; a save may have completed."
            : cause instanceof Error
              ? cause.message
              : "Target request failed",
        );
    } finally {
      window.clearTimeout(timer);
      if (controller.current === current) setBusy(false);
    }
  }
  async function load(secret: string) {
    await perform(async (signal) => {
      const list = parseTargets(
        await targetRequest(secret, "GET", null, signal),
      );
      if (signal.aborted) return;
      setToken(secret);
      setTokenInput("");
      setTargets(list);
      setRemoving(null);
    });
  }
  function lock() {
    controller.current?.abort();
    controller.current = null;
    setToken("");
    setTokenInput("");
    setTargets(null);
    setBusy(false);
    setEditing(null);
    setRemoving(null);
    setForm(empty);
    setError(null);
    setMessage(null);
  }
  function edit(target: MonitoringTarget) {
    setEditing(target.id);
    setRemoving(null);
    setForm({
      type: target.type,
      address: target.address,
      enabled: target.enabled,
      interval_ms: target.interval_ms,
      timeout_ms: target.timeout_ms,
    });
  }
  async function save(event: FormEvent) {
    event.preventDefault();
    await perform(async (signal) => {
      const saved = parseTarget(
        await targetRequest(
          token,
          editing ? "PUT" : "POST",
          editing,
          signal,
          form,
        ),
      );
      if (signal.aborted) return;
      setTargets((previous) => [
        ...(previous ?? []).filter((t) => t.id !== saved.id),
        saved,
      ]);
      setForm(empty);
      setEditing(null);
      setMessage(
        "Target saved. Scheduling changes apply after the current cycle; an in-flight probe may still finish.",
      );
    });
  }
  async function remove(id: string) {
    await perform(async (signal) => {
      const result = await targetRequest(token, "DELETE", id, signal);
      if (signal.aborted) return;
      if (
        typeof result !== "object" ||
        result === null ||
        !("deleted" in result) ||
        result.deleted !== true
      )
        throw new Error("Invalid deletion response");
      setTargets((previous) => (previous ?? []).filter((t) => t.id !== id));
      setRemoving(null);
      if (editing === id) {
        setEditing(null);
        setForm(empty);
      }
      setMessage(
        "Target removed. Historical measurements and incidents remain available.",
      );
    });
  }
  async function toggle(target: MonitoringTarget) {
    const { id, ...input } = target;
    await perform(async (signal) => {
      const saved = parseTarget(
        await targetRequest(token, "PUT", id, signal, {
          ...input,
          enabled: !input.enabled,
        }),
      );
      if (signal.aborted) return;
      setTargets((previous) =>
        (previous ?? []).map((t) => (t.id === id ? saved : t)),
      );
      setMessage(
        `Target ${saved.enabled ? "enabled" : "disabled"}. An in-flight probe may still finish.`,
      );
    });
  }

  return (
    <div className="space-y-6">
      <p className="max-w-3xl text-sm leading-6 text-gray-400">
        Manage TCP endpoints and DNS domains you are authorized to monitor. Up
        to 32 targets; intervals from 5 seconds to 1 hour. Disabling or removing
        a target stops future scheduling and preserves its history. Unmeasured
        time remains unknown.
      </p>
      {!token ? (
        <form
          onSubmit={(event) => {
            event.preventDefault();
            void load(tokenInput);
          }}
          className="max-w-xl space-y-4 rounded-xl border border-white/10 bg-[#111820] p-5"
        >
          <label className="block space-y-2 text-sm text-gray-300">
            <span>Admin token</span>
            <input
              aria-label="Admin token"
              type="password"
              autoComplete="off"
              required
              minLength={32}
              maxLength={256}
              value={tokenInput}
              onChange={(e) => setTokenInput(e.target.value)}
              className={fieldClass}
            />
          </label>
          <p className="text-xs leading-6 text-gray-400">
            Set PACKETLENS_ADMIN_TOKEN privately on the backend and restart it
            to enable management. The token stays in this page&apos;s memory and
            is cleared when you leave or lock it.
          </p>
          <button className={buttonClass} disabled={busy} type="submit">
            Unlock target management
          </button>
        </form>
      ) : (
        <>
          <div className="flex flex-wrap gap-3">
            <button
              className={buttonClass}
              disabled={busy}
              onClick={() => void load(token)}
            >
              Refresh targets
            </button>
            <button className={buttonClass} onClick={lock}>
              Lock management
            </button>
          </div>
          <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(280px,1fr)]">
            <section
              className="min-w-0 space-y-3"
              aria-label="Configured targets"
            >
              <h2 className="text-lg font-medium">
                Configured targets ({targets?.length ?? 0}/32)
              </h2>
              {targets?.length === 0 && (
                <p className="rounded-xl border border-white/10 p-5 text-sm text-gray-400">
                  No targets configured. Add a target to resume monitoring.
                  Defaults will not be restored automatically.
                </p>
              )}
              {targets?.map((target) => (
                <article
                  key={target.id}
                  className="rounded-xl border border-white/10 bg-[#111820] p-5"
                >
                  <div className="flex flex-wrap justify-between gap-3">
                    <h3 className="min-w-0 break-all font-medium">
                      {target.type.toUpperCase()} · {target.address}
                    </h3>
                    <span
                      className={`text-xs ${target.enabled ? "text-teal-300" : "text-gray-400"}`}
                    >
                      {target.enabled ? "Enabled" : "Disabled"}
                    </span>
                  </div>
                  <p className="mt-2 text-xs text-gray-400">
                    Every {target.interval_ms / 1000}s · timeout{" "}
                    {target.timeout_ms / 1000}s
                  </p>
                  <div className="mt-4 flex flex-wrap gap-3">
                    <button
                      className={buttonClass}
                      disabled={busy}
                      onClick={() => edit(target)}
                      aria-label={`Edit ${target.address}`}
                    >
                      Edit
                    </button>
                    <button
                      className={buttonClass}
                      disabled={busy || editing === target.id}
                      onClick={() => void toggle(target)}
                      aria-label={`${target.enabled ? "Disable" : "Enable"} ${target.address}`}
                    >
                      {target.enabled ? "Disable" : "Enable"}
                    </button>
                    <button
                      className={buttonClass}
                      disabled={busy}
                      onClick={() => setRemoving(target.id)}
                      aria-label={`Remove ${target.address}`}
                    >
                      Remove
                    </button>
                  </div>
                  {removing === target.id && (
                    <div className="mt-4 space-y-3 text-sm text-gray-300">
                      <p>
                        Remove {target.address}? Historical records will be
                        retained.
                      </p>
                      <div className="flex gap-3">
                        <button
                          className={buttonClass}
                          disabled={busy}
                          onClick={() => void remove(target.id)}
                        >
                          Confirm removal
                        </button>
                        <button
                          className={buttonClass}
                          disabled={busy}
                          onClick={() => setRemoving(null)}
                        >
                          Keep target
                        </button>
                      </div>
                    </div>
                  )}
                </article>
              ))}
            </section>
            <form
              onSubmit={(event) => void save(event)}
              className="space-y-4 rounded-xl border border-white/10 bg-[#111820] p-5"
              aria-label="Target editor"
            >
              <h2 className="text-lg font-medium">
                {editing ? "Edit target" : "Add target"}
              </h2>
              <label className="block space-y-2 text-sm text-gray-300">
                <span>Probe type</span>
                <select
                  aria-label="Target probe type"
                  disabled={busy}
                  value={form.type}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      type: e.target.value as TargetInput["type"],
                    })
                  }
                  className={fieldClass}
                >
                  <option value="tcp">TCP</option>
                  <option value="dns">DNS</option>
                </select>
              </label>
              <label className="block space-y-2 text-sm text-gray-300">
                <span>
                  {form.type === "tcp" ? "Host and port" : "Domain name"}
                </span>
                <input
                  aria-label="Target address"
                  disabled={busy}
                  value={form.address}
                  onChange={(e) =>
                    setForm({ ...form, address: e.target.value })
                  }
                  required
                  maxLength={512}
                  placeholder={
                    form.type === "tcp" ? "example.com:443" : "example.com"
                  }
                  className={fieldClass}
                />
              </label>
              <label className="block space-y-2 text-sm text-gray-300">
                <span>Interval (seconds)</span>
                <input
                  aria-label="Probe interval seconds"
                  disabled={busy}
                  type="number"
                  min={5}
                  max={3600}
                  step={1}
                  required
                  value={form.interval_ms / 1000}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      interval_ms: Number(e.target.value) * 1000,
                    })
                  }
                  className={fieldClass}
                />
              </label>
              <label className="block space-y-2 text-sm text-gray-300">
                <span>Timeout (seconds)</span>
                <input
                  aria-label="Probe timeout seconds"
                  disabled={busy}
                  type="number"
                  min={0.1}
                  max={Math.min(30, form.interval_ms / 1000)}
                  step={0.1}
                  required
                  value={form.timeout_ms / 1000}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      timeout_ms: Math.round(Number(e.target.value) * 1000),
                    })
                  }
                  className={fieldClass}
                />
              </label>
              <label className="flex gap-2 text-sm text-gray-300">
                <input
                  disabled={busy}
                  type="checkbox"
                  checked={form.enabled}
                  onChange={(e) =>
                    setForm({ ...form, enabled: e.target.checked })
                  }
                />
                Enabled
              </label>
              <div className="flex gap-3">
                <button
                  className={buttonClass}
                  disabled={busy || (!editing && (targets?.length ?? 0) >= 32)}
                  type="submit"
                >
                  {editing ? "Save changes" : "Add target"}
                </button>
                {editing && (
                  <button
                    className={buttonClass}
                    disabled={busy}
                    type="button"
                    onClick={() => {
                      setEditing(null);
                      setForm(empty);
                    }}
                  >
                    Cancel edit
                  </button>
                )}
              </div>
            </form>
          </div>
        </>
      )}
      {busy && (
        <p role="status" className="text-sm text-gray-400">
          Updating target configuration...
        </p>
      )}
      {error && (
        <p role="alert" className="text-sm text-amber-300">
          {error}
        </p>
      )}
      {message && (
        <p role="status" className="text-sm text-teal-300">
          {message}
        </p>
      )}
    </div>
  );
}
