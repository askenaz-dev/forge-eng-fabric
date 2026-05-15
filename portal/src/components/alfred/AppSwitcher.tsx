"use client";

import * as Popover from "@radix-ui/react-popover";
import { useLang } from "@/components/providers/LangProvider";
import { ChevDown } from "@/components/icons";

export type AppEntry = {
  id: string;
  name: string;
  slug: string;
  lastActivity?: string;
};

/**
 * Friendly view App switcher (alfred-console-redesign requirement 1.5).
 *
 * - Multiple apps → clickable popover switcher.
 * - Single app   → static label (no popover affordance).
 */
export function AppSwitcher({
  apps,
  activeAppId,
  onSwitch,
}: {
  apps: AppEntry[];
  activeAppId: string | null;
  onSwitch: (appId: string) => void;
}) {
  const { t } = useLang();
  const visible = apps.filter((a) => !a.slug.startsWith("_"));
  const activeApp = visible.find((a) => a.id === activeAppId) ?? visible[0] ?? null;

  if (visible.length <= 1) {
    return (
      <span className="alfred-app-label" aria-label={t("alfred_app_switcher_label")}>
        {t("alfred_app_switcher_label")} {activeApp?.name ?? "—"}
      </span>
    );
  }

  return (
    <Popover.Root>
      <Popover.Trigger asChild>
        <button
          type="button"
          className="alfred-app-switcher-btn"
          aria-label={t("alfred_app_switcher_aria")}
        >
          <span className="alfred-app-switcher-label">
            {t("alfred_app_switcher_label")} {activeApp?.name ?? "—"}
          </span>
          <ChevDown className="alfred-app-switcher-chev" aria-hidden />
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content className="pop" align="start" sideOffset={6} collisionPadding={12}>
          <ul className="alfred-app-switcher-list" role="listbox" aria-label={t("alfred_app_switcher_aria")}>
            {visible.map((app) => (
              <li key={app.id} role="option" aria-selected={app.id === activeAppId}>
                <button
                  type="button"
                  className={`pop-item ${app.id === activeAppId ? "active" : ""}`}
                  onClick={() => onSwitch(app.id)}
                >
                  <span className="font-medium">{app.name}</span>
                  {app.lastActivity && (
                    <small style={{ color: "var(--fg-3)", fontFamily: "var(--f-mono)" }}>
                      {t("alfred_app_switcher_last")}: {app.lastActivity}
                    </small>
                  )}
                </button>
              </li>
            ))}
          </ul>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
