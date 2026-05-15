"use client";

import { useEffect, useId, useMemo, useRef, useState } from "react";

type Member = { subject: string; workspaces?: string[] };
type PlatformUser = { subject: string; username?: string; email?: string };
type Suggestion = {
  value: string;
  label: string;
  hint?: string;
  source: "bu" | "platform";
};

export type OwnersMultiSelectProps = {
  name: string;
  /** Pre-selected owners. */
  defaultValue?: string[];
  /** Business unit ID used to fetch suggestable members. When empty, the
   *  dropdown stays empty but the user can still type and create chips. */
  businessUnitId?: string;
  required?: boolean;
  placeholder?: string;
  /** Controlled mode: when `value` is provided, the component is controlled
   *  and reports changes via `onChange`. */
  value?: string[];
  onChange?: (next: string[]) => void;
};

export function OwnersMultiSelect({
  name,
  defaultValue = [],
  businessUnitId,
  required,
  placeholder = "Add owners…",
  value: controlledValue,
  onChange,
}: OwnersMultiSelectProps) {
  const controlled = controlledValue !== undefined;
  const [internal, setInternal] = useState<string[]>(defaultValue);
  const selected = controlled ? (controlledValue as string[]) : internal;
  const setSelected = (next: string[]) => {
    if (!controlled) setInternal(next);
    onChange?.(next);
  };

  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [members, setMembers] = useState<Member[]>([]);
  const [platformUsers, setPlatformUsers] = useState<PlatformUser[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const listboxId = useId();

  // Platform users are the global pool — independent of which BU the user is
  // about to create a workspace in. Fetched once on mount.
  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const resp = await fetch(`/api/platform/users`, { cache: "no-store" });
        if (!resp.ok) throw new Error(`${resp.status}`);
        const body = await resp.json();
        if (!alive) return;
        setPlatformUsers(Array.isArray(body?.users) ? body.users : []);
      } catch {
        if (!alive) return;
        setPlatformUsers([]);
        // Platform-user lookup failing is non-fatal — the user can still type
        // a value, and BU members may still load.
      }
    })();
    return () => {
      alive = false;
    };
  }, []);

  // Fetch BU members when the BU id changes (debounced so we don't fire a
  // request on every keystroke while the user types the UUID).
  useEffect(() => {
    if (!businessUnitId) {
      setMembers([]);
      setLoadError(null);
      return;
    }
    let alive = true;
    const timer = setTimeout(async () => {
      try {
        const resp = await fetch(
          `/api/business-units/${encodeURIComponent(businessUnitId)}/members`,
          { cache: "no-store" },
        );
        if (!resp.ok) throw new Error(`${resp.status}`);
        const body = await resp.json();
        if (!alive) return;
        setMembers(Array.isArray(body?.members) ? body.members : []);
        setLoadError(null);
      } catch (err) {
        if (!alive) return;
        setMembers([]);
        setLoadError(err instanceof Error ? err.message : "member lookup failed");
      }
    }, 300);
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [businessUnitId]);

  // Close the dropdown when clicking outside.
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (!containerRef.current) return;
      if (!containerRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  const trimmedQuery = query.trim();
  const lowerQuery = trimmedQuery.toLowerCase();

  const suggestions = useMemo(() => {
    const selectedSet = new Set(selected);
    const seen = new Set<string>();
    const filtered: Suggestion[] = [];

    // BU members first — they already have a relationship to *some* workspace
    // in this business unit, so they're the most likely picks.
    for (const m of members) {
      if (selectedSet.has(m.subject) || seen.has(m.subject)) continue;
      const label = m.subject;
      if (lowerQuery && !label.toLowerCase().includes(lowerQuery)) continue;
      const count = m.workspaces?.length ?? 0;
      filtered.push({
        value: m.subject,
        label,
        hint: count > 0 ? `${count} workspace${count === 1 ? "" : "s"}` : undefined,
        source: "bu",
      });
      seen.add(m.subject);
    }

    // Then everyone else who has ever signed in to the platform.
    for (const u of platformUsers) {
      const value = u.username || u.email || u.subject;
      if (!value || selectedSet.has(value) || seen.has(value)) continue;
      const matches =
        !lowerQuery ||
        value.toLowerCase().includes(lowerQuery) ||
        (u.email?.toLowerCase().includes(lowerQuery) ?? false);
      if (!matches) continue;
      filtered.push({
        value,
        label: value,
        hint: u.email && u.email !== value ? u.email : undefined,
        source: "platform",
      });
      seen.add(value);
    }

    const canCreate =
      trimmedQuery !== "" &&
      !selectedSet.has(trimmedQuery) &&
      !filtered.some((s) => s.value === trimmedQuery);
    return { filtered, canCreate };
  }, [members, platformUsers, selected, lowerQuery, trimmedQuery]);

  const totalOptions = suggestions.filtered.length + (suggestions.canCreate ? 1 : 0);

  useEffect(() => {
    setActiveIndex(0);
  }, [query, members.length, platformUsers.length]);

  function addChip(subject: string) {
    const value = subject.trim();
    if (!value) return;
    if (selected.includes(value)) return;
    setSelected([...selected, value]);
    setQuery("");
    setActiveIndex(0);
  }

  function removeChip(subject: string) {
    setSelected(selected.filter((s) => s !== subject));
  }

  function commitActive() {
    if (suggestions.filtered.length > 0 && activeIndex < suggestions.filtered.length) {
      addChip(suggestions.filtered[activeIndex].value);
      return;
    }
    if (suggestions.canCreate) {
      addChip(trimmedQuery);
    }
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      if (open && totalOptions > 0) {
        commitActive();
      } else if (trimmedQuery) {
        addChip(trimmedQuery);
      }
      return;
    }
    if (e.key === "Tab" && trimmedQuery && totalOptions > 0) {
      e.preventDefault();
      commitActive();
      return;
    }
    if (e.key === "Backspace" && query === "" && selected.length > 0) {
      e.preventDefault();
      removeChip(selected[selected.length - 1]);
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
      setActiveIndex((i) => (totalOptions === 0 ? 0 : Math.min(i + 1, totalOptions - 1)));
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => Math.max(i - 1, 0));
      return;
    }
    if (e.key === "Escape") {
      setOpen(false);
      return;
    }
  }

  function handlePaste(e: React.ClipboardEvent<HTMLInputElement>) {
    const text = e.clipboardData.getData("text");
    if (!text.includes(",")) return;
    e.preventDefault();
    const fresh = text
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
    if (fresh.length === 0) return;
    const dedup = new Set(selected);
    for (const s of fresh) dedup.add(s);
    setSelected(Array.from(dedup));
    setQuery("");
  }

  const hiddenValue = selected.join(",");

  return (
    <div ref={containerRef} style={{ position: "relative" }}>
      <div
        className="top-search"
        style={{
          width: "100%",
          height: "auto",
          minHeight: 36,
          padding: "4px 6px",
          flexWrap: "wrap",
          cursor: "text",
          alignItems: "center",
        }}
        onClick={() => inputRef.current?.focus()}
      >
        {selected.map((s) => (
          <span
            key={s}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 4,
              background: "var(--bg-hover, rgba(0,0,0,0.06))",
              border: "1px solid var(--border)",
              borderRadius: "var(--r-2)",
              padding: "2px 4px 2px 8px",
              fontSize: 12,
              color: "var(--fg)",
            }}
          >
            {s}
            <button
              type="button"
              aria-label={`Remove ${s}`}
              onClick={(e) => {
                e.stopPropagation();
                removeChip(s);
              }}
              style={{
                background: "transparent",
                border: "none",
                cursor: "pointer",
                padding: "0 4px",
                fontSize: 14,
                lineHeight: 1,
                color: "var(--fg-2)",
              }}
            >
              ×
            </button>
          </span>
        ))}
        <input
          ref={inputRef}
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={handleKeyDown}
          onPaste={handlePaste}
          placeholder={selected.length === 0 ? placeholder : ""}
          aria-autocomplete="list"
          aria-controls={listboxId}
          aria-expanded={open}
          role="combobox"
          style={{
            flex: 1,
            minWidth: 100,
            background: "transparent",
            border: "none",
            outline: "none",
            color: "var(--fg)",
            fontSize: 13,
            height: 24,
            padding: "0 4px",
          }}
        />
      </div>
      <input
        type="hidden"
        name={name}
        value={hiddenValue}
        required={required && selected.length === 0}
      />
      {open && (totalOptions > 0 || loadError || platformUsers.length === 0) && (
        <ul
          id={listboxId}
          role="listbox"
          style={{
            position: "absolute",
            top: "100%",
            left: 0,
            right: 0,
            marginTop: 4,
            zIndex: 20,
            background: "var(--bg-card)",
            border: "1px solid var(--border)",
            borderRadius: "var(--r-2)",
            boxShadow: "var(--shadow, 0 4px 16px rgba(0,0,0,0.12))",
            maxHeight: 220,
            overflowY: "auto",
            padding: 4,
            margin: 0,
            listStyle: "none",
            fontSize: 13,
          }}
        >
          {suggestions.filtered.map((s, idx) => {
            const active = idx === activeIndex;
            return (
              <li
                key={`${s.source}:${s.value}`}
                role="option"
                aria-selected={active}
                onMouseDown={(e) => {
                  e.preventDefault();
                  addChip(s.value);
                }}
                onMouseEnter={() => setActiveIndex(idx)}
                style={{
                  padding: "6px 8px",
                  borderRadius: "var(--r-1)",
                  cursor: "pointer",
                  background: active ? "var(--bg-hover)" : "transparent",
                  display: "flex",
                  justifyContent: "space-between",
                  gap: 8,
                }}
              >
                <span style={{ color: "var(--fg)" }}>{s.label}</span>
                {s.hint && (
                  <span style={{ color: "var(--fg-3)", fontSize: 11 }}>{s.hint}</span>
                )}
              </li>
            );
          })}
          {suggestions.canCreate && (
            <li
              role="option"
              aria-selected={activeIndex === suggestions.filtered.length}
              onMouseDown={(e) => {
                e.preventDefault();
                addChip(trimmedQuery);
              }}
              onMouseEnter={() => setActiveIndex(suggestions.filtered.length)}
              style={{
                padding: "6px 8px",
                borderRadius: "var(--r-1)",
                cursor: "pointer",
                background:
                  activeIndex === suggestions.filtered.length ? "var(--bg-hover)" : "transparent",
                color: "var(--fg-2)",
                fontStyle: "italic",
              }}
            >
              Add &ldquo;{trimmedQuery}&rdquo;
            </li>
          )}
          {!suggestions.filtered.length && !suggestions.canCreate && (
            <li
              role="option"
              aria-disabled
              style={{ padding: "6px 8px", color: "var(--fg-3)", fontSize: 12 }}
            >
              {loadError
                ? `Directory unavailable (${loadError}).`
                : "No matching users — type a name and press Enter to add one."}
            </li>
          )}
        </ul>
      )}
    </div>
  );
}
