"use client";

import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { Check, Monitor, Moon, Sun } from "../icons";
import { useTheme } from "../providers/ThemeProvider";
import { useLang } from "../providers/LangProvider";
import { useToast } from "../providers/ToastProvider";
import type { ThemePref } from "@/lib/prefs";

export function ThemeMenu() {
  const { pref, effective, setPref } = useTheme();
  const { t } = useLang();
  const toast = useToast();
  const Icon = effective === "dark" ? Moon : Sun;

  function choose(next: ThemePref) {
    if (next === pref) return;
    setPref(next);
    toast.success(t("toast_theme"));
  }

  const items: Array<{ id: ThemePref; icon: typeof Sun; label: string; hint?: string }> = [
    { id: "light",  icon: Sun,     label: t("theme_light") },
    { id: "dark",   icon: Moon,    label: t("theme_dark") },
    { id: "system", icon: Monitor, label: t("theme_system"), hint: t("theme_system_hint") },
  ];

  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <button className="icon-btn" aria-label={t("tb_theme")}>
          <Icon />
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content className="pop" align="end" sideOffset={6}>
          {items.map((item) => {
            const Glyph = item.icon;
            const active = pref === item.id;
            return (
              <DropdownMenu.Item
                key={item.id}
                className={`pop-item${active ? " active" : ""}`}
                onSelect={() => choose(item.id)}
              >
                <Glyph className="lead" />
                <span>{item.label}</span>
                {item.hint && <small>{item.hint}</small>}
                {active && (
                  <span className="check">
                    <Check style={{ width: 13, height: 13 }} />
                  </span>
                )}
              </DropdownMenu.Item>
            );
          })}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}
