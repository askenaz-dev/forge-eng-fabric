"use client";

import * as Dialog from "@radix-ui/react-dialog";
import { Command } from "cmdk";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { useCommandPalette } from "../providers/CommandPaletteProvider";
import { useLang } from "../providers/LangProvider";
import { useTheme } from "../providers/ThemeProvider";
import { useDensity } from "../providers/DensityProvider";
import { useToast } from "../providers/ToastProvider";
import {
  PALETTE_GROUP_LABELS,
  PaletteAction,
  PaletteResult,
  PaletteSourceResponse,
} from "./types";

export function CommandPalette() {
  const { open, hide } = useCommandPalette();
  const { t, lang, setLang } = useLang();
  const { setPref } = useTheme();
  const { setDensity } = useDensity();
  const toast = useToast();
  const router = useRouter();
  const searchParams = useSearchParams();
  const [query, setQuery] = useState("");
  const [responses, setResponses] = useState<PaletteSourceResponse[]>([]);
  const [loading, setLoading] = useState(false);

  // alfred-console-redesign §8.8: detect Friendly view; palette / command
  // disabled. The `/` keystroke is NOT globally captured in Friendly view.
  const isFriendly =
    typeof window !== "undefined" &&
    window.location.pathname.startsWith("/alfred") &&
    (searchParams.get("view") === "friendly" ||
      (!searchParams.get("view") &&
        typeof localStorage !== "undefined" &&
        localStorage.getItem("alfred_view") !== "advanced"));

  // Reset state when reopened.
  useEffect(() => {
    if (!open) {
      setQuery("");
      setResponses([]);
    }
  }, [open]);

  // Fetch on open and on debounced query change.
  useEffect(() => {
    if (!open) return;
    const handle = setTimeout(async () => {
      setLoading(true);
      try {
        const url = `/api/command-palette/search?q=${encodeURIComponent(query)}`;
        const r = await fetch(url, { cache: "no-store" });
        if (!r.ok) throw new Error(`palette search ${r.status}`);
        const payload = (await r.json()) as { sources: PaletteSourceResponse[] };
        setResponses(payload.sources);
      } catch {
        setResponses([]);
      } finally {
        setLoading(false);
      }
    }, query ? 120 : 0);
    return () => clearTimeout(handle);
  }, [open, query]);

  const totalCount = useMemo(
    () => responses.reduce((sum, r) => sum + r.results.length, 0),
    [responses],
  );

  async function pick(result: PaletteResult) {
    void emitInvocation(result, query);
    if (result.hrefOrAction.kind === "navigate") {
      router.push(result.hrefOrAction.href);
      hide();
      return;
    }
    await runAction(result.hrefOrAction.action);
  }

  async function runAction(action: PaletteAction) {
    switch (action.type) {
      case "theme":
        setPref(action.theme);
        toast.success(t("toast_theme"));
        break;
      case "density":
        setDensity(action.density);
        toast.success(t("toast_density"));
        break;
      case "lang":
        setLang(action.lang);
        toast.success(action.lang === "es" ? t("toast_lang") : t("toast_lang_en"));
        break;
      case "sidebar": {
        const cur = localStorage.getItem("forge_sidebar");
        const next = cur === "collapsed" ? "expanded" : "collapsed";
        localStorage.setItem("forge_sidebar", next);
        document.body.classList.toggle("app--collapsed", next === "collapsed");
        break;
      }
      case "sign-out":
        router.push("/api/auth/signout");
        break;
      case "workspace":
        await fetch("/api/workspace/active", {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ tenant: action.tenant, workspace: action.workspace }),
        }).catch(() => undefined);
        toast.success(t("toast_workspace"));
        router.refresh();
        break;
      case "forge-command":
        // alfred-console-redesign §8.3-8.4: show deprecation toast for /openspec
        if (action.deprecated) {
          toast.error(t("alfred_cmd_deprecated_toast"));
          // Emit deprecated_alias audit event (§8.3).
          void fetch("/api/command-palette/audit", {
            method: "POST",
            headers: { "content-type": "application/json" },
            body: JSON.stringify({
              source: "palette",
              target_id: `openspec.${action.subcommand}`,
              query,
              event_type: "alfred.command.deprecated_alias.v1",
            }),
            keepalive: true,
          });
        }
        router.push(`/alfred?view=advanced&cmd=forge.${action.subcommand}`);
        break;
    }
    hide();
  }

  // alfred-console-redesign §8.1-8.2: /forge (primary) and /openspec (deprecated alias).
  // §8.8: hidden when in Friendly view.
  const forgeActions = useMemo<PaletteResult[]>(() => {
    if (isFriendly) return [];
    return [
      {
        id: "forge.new",
        source: "nav",
        title: "/forge new",
        subtitle: "Create a new specification",
        hrefOrAction: { kind: "action", action: { type: "forge-command", subcommand: "new" } },
      },
      {
        id: "forge.list",
        source: "nav",
        title: "/forge list",
        subtitle: "List specifications",
        hrefOrAction: { kind: "action", action: { type: "forge-command", subcommand: "list" } },
      },
      {
        id: "openspec.new",
        source: "nav",
        title: "/openspec new (deprecated)",
        subtitle: "Use /forge new instead",
        hrefOrAction: { kind: "action", action: { type: "forge-command", subcommand: "new", deprecated: true } },
      },
    ];
  }, [isFriendly]);

  const localActions = useMemo<PaletteResult[]>(() => {
    return [
      {
        id: "action.theme.light",
        source: "actions",
        title: t("cmd_action_theme_light"),
        hrefOrAction: { kind: "action", action: { type: "theme", theme: "light" } },
      },
      {
        id: "action.theme.dark",
        source: "actions",
        title: t("cmd_action_theme_dark"),
        hrefOrAction: { kind: "action", action: { type: "theme", theme: "dark" } },
      },
      {
        id: "action.theme.system",
        source: "actions",
        title: t("cmd_action_theme_system"),
        hrefOrAction: { kind: "action", action: { type: "theme", theme: "system" } },
      },
      {
        id: "action.lang.es",
        source: "actions",
        title: t("cmd_action_lang_es"),
        hrefOrAction: { kind: "action", action: { type: "lang", lang: "es" } },
      },
      {
        id: "action.lang.en",
        source: "actions",
        title: t("cmd_action_lang_en"),
        hrefOrAction: { kind: "action", action: { type: "lang", lang: "en" } },
      },
      {
        id: "action.sidebar.toggle",
        source: "actions",
        title: t("cmd_action_sidebar"),
        hrefOrAction: { kind: "action", action: { type: "sidebar" } },
      },
      {
        id: "action.sign-out",
        source: "actions",
        title: t("cmd_action_sign_out"),
        hrefOrAction: { kind: "action", action: { type: "sign-out" } },
      },
    ];
  }, [t]);

  // Merge server responses, forge commands, and local actions.
  const merged: PaletteSourceResponse[] = useMemo(() => {
    const filtered = query.startsWith(">") ? [] : responses;
    const actionsResp: PaletteSourceResponse = {
      source: "actions",
      status: "ok",
      results: localActions,
    };
    // Inject /forge entries into the nav group (§8.1).
    const navResp: PaletteSourceResponse = {
      source: "nav",
      status: "ok",
      results: forgeActions,
    };
    const existing = filtered.find((r) => r.source === "nav");
    const mergedFiltered = existing
      ? filtered.map((r) =>
          r.source === "nav" ? { ...r, results: [...forgeActions, ...r.results] } : r,
        )
      : forgeActions.length > 0
        ? [navResp, ...filtered]
        : filtered;
    return [...mergedFiltered, actionsResp];
  }, [responses, localActions, forgeActions, query]);

  // alfred-console-redesign §8.8: in Friendly view, don't open palette on `/`.
  // This is enforced in the CommandPaletteProvider trigger — handled here by
  // checking isFriendly in the keyboard listener so the key is delivered as text.
  useEffect(() => {
    if (!isFriendly) return;
    function blockSlashOpen(e: KeyboardEvent) {
      if (e.key === "/" && !["INPUT", "TEXTAREA"].includes((e.target as HTMLElement)?.tagName)) {
        // Friendly view: don't open palette, deliver the keystroke as text.
        e.stopPropagation();
      }
    }
    window.addEventListener("keydown", blockSlashOpen, { capture: true });
    return () => window.removeEventListener("keydown", blockSlashOpen, { capture: true });
  }, [isFriendly]);

  return (
    <Dialog.Root open={open} onOpenChange={(o) => (o ? null : hide())}>
      <Dialog.Portal>
        <Dialog.Overlay className="scrim" />
        <Dialog.Content
          className="cmdk-root"
          aria-label="command palette"
          aria-describedby={undefined}
        >
          <Dialog.Title className="sr-only">{t("tb_search")}</Dialog.Title>
          <Command label={t("tb_search")} loop shouldFilter>
            <div className="cmdk-input-row">
              <Command.Input
                className="cmdk-input"
                placeholder={t("cmd_placeholder")}
                value={query}
                onValueChange={setQuery}
                autoFocus
              />
              <span
                aria-live="polite"
                style={{ fontFamily: "var(--f-mono)", fontSize: 10.5, color: "var(--fg-3)" }}
              >
                {t("cmd_results_count", { n: totalCount })}
              </span>
            </div>
            <Command.List className="cmdk-list">
              <Command.Empty className="cmdk-empty">
                {loading ? "…" : t("cmd_no_results")}
              </Command.Empty>
              {PALETTE_GROUP_LABELS.map((g) => {
                const group = merged.find((r) => r.source === g.source);
                if (!group || group.results.length === 0) return null;
                return (
                  <Command.Group
                    key={g.source}
                    heading={
                      <span className="cmdk-group-heading">
                        {t(g.labelKey)}
                        {group.status === "unreachable" && (
                          <span style={{ marginLeft: 6, color: "var(--rust)" }}>
                            ({t("cmd_unavailable")})
                          </span>
                        )}
                      </span>
                    }
                  >
                    {group.results.map((r) => (
                      <Command.Item
                        key={r.id}
                        value={`${r.title} ${r.subtitle ?? ""} ${r.source}`}
                        onSelect={() => pick(r)}
                        className="cmdk-item"
                      >
                        <span className="source">{r.source}</span>
                        <span>
                          <div>{r.title}</div>
                          {r.subtitle && (
                            <small style={{ color: "var(--fg-3)", fontFamily: "var(--f-mono)" }}>
                              {r.subtitle}
                            </small>
                          )}
                        </span>
                      </Command.Item>
                    ))}
                  </Command.Group>
                );
              })}
            </Command.List>
          </Command>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

async function emitInvocation(result: PaletteResult, query: string) {
  try {
    await fetch("/api/command-palette/audit", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        source: result.source,
        target_id: result.id,
        query,
      }),
      keepalive: true,
    });
  } catch {
    // ignored — best-effort audit
  }
}
