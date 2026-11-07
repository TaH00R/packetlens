"use client";
import { useId, useState, type ReactNode } from "react";
interface Tab {
  id: string;
  label: string;
  content: ReactNode;
}
export default function DataTabs({
  label,
  tabs,
}: {
  label: string;
  tabs: Tab[];
}) {
  const prefix = useId();
  const [selected, setSelected] = useState(tabs[0]?.id);
  const active = tabs.find((tab) => tab.id === selected) ?? tabs[0];
  if (!active) return null;
  return (
    <div className="min-w-0 space-y-6">
      <div
        role="tablist"
        aria-label={label}
        className="flex flex-wrap gap-1 rounded-xl border border-white/[0.07] bg-[#0D141D] p-1.5"
      >
        {tabs.map((tab, index) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            id={`${prefix}-${tab.id}`}
            aria-controls={`${prefix}-panel-${tab.id}`}
            aria-selected={active.id === tab.id}
            tabIndex={active.id === tab.id ? 0 : -1}
            onClick={() => setSelected(tab.id)}
            onKeyDown={(event) => {
              const next =
                event.key === "ArrowRight"
                  ? (index + 1) % tabs.length
                  : event.key === "ArrowLeft"
                    ? (index - 1 + tabs.length) % tabs.length
                    : event.key === "Home"
                      ? 0
                      : event.key === "End"
                        ? tabs.length - 1
                        : -1;
              if (next < 0) return;
              event.preventDefault();
              setSelected(tabs[next].id);
              document.getElementById(`${prefix}-${tabs[next].id}`)?.focus();
            }}
            className={`rounded-lg px-4 py-2.5 text-xs font-medium transition-colors ${active.id === tab.id ? "bg-teal-500/10 text-teal-300 shadow-sm ring-1 ring-teal-500/20" : "text-slate-400 hover:bg-white/5 hover:text-slate-200"}`}
          >
            {tab.label}
          </button>
        ))}
      </div>
      <div
        role="tabpanel"
        id={`${prefix}-panel-${active.id}`}
        aria-labelledby={`${prefix}-${active.id}`}
        tabIndex={0}
        className="min-w-0 outline-none"
      >
        {active.content}
      </div>
    </div>
  );
}
