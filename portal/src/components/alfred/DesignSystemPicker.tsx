"use client";

/**
 * DesignSystemPicker — shared catalog picker used by Alfred's Friendly view
 * (Nueva App card) and by /alfred/wizard's new-App branch.
 *
 * alfred-design-system-picker (D2): presentational only — props carry the
 * catalog, the selection state, and two callbacks for "continue with
 * selected" and "skip". The host owns API calls (catalog fetch + atomic
 * POST). This keeps the component portable and trivially testable.
 *
 * Layout: single column at widths <= 720px, two columns above. Each card
 * shows `manifest.screenshots.light` + `manifest.screenshots.dark`,
 * `name` in Instrument Serif italic and `use_case` in Geist. Skip is a
 * secondary action distinct from Continue (D3).
 */

import { useLang } from "@/components/providers/LangProvider";

export type DesignSystemEntry = {
  asset_id: string;
  version: string;
  name: string;
  manifest?: {
    use_case?: string;
    screenshots?: { light?: string; dark?: string };
  };
  built_in_template?: boolean;
  eval_scores?: Record<string, number>;
};

export interface DesignSystemPickerProps {
  catalog: DesignSystemEntry[];
  selectedRef: string | null;
  onSelect: (ref: string) => void;
  onContinue: () => void;
  onSkip: () => void;
  loading?: boolean;
  loadError?: boolean;
  showNewBadge?: boolean;
}

const SKELETON_KEYS = ["sk-0", "sk-1", "sk-2", "sk-3"];

function refOf(entry: DesignSystemEntry): string {
  return `${entry.asset_id}@${entry.version}`;
}

export function DesignSystemPicker(props: DesignSystemPickerProps) {
  const { t } = useLang();
  const {
    catalog,
    selectedRef,
    onSelect,
    onContinue,
    onSkip,
    loading = false,
    loadError = false,
    showNewBadge = false,
  } = props;

  const showEmptyState = !loading && (loadError || catalog.length === 0);

  return (
    <section
      aria-labelledby="ds-step-title"
      data-testid="ds-picker"
      className="space-y-4"
    >
      <header className="flex items-baseline justify-between gap-3">
        <div>
          <h2
            id="ds-step-title"
            className="text-xl"
            style={{ fontFamily: "var(--font-instrument-serif), serif", fontStyle: "italic" }}
          >
            {t("alfred_ds_step_title")}
          </h2>
          <p className="text-sm" style={{ color: "var(--fg-2)" }}>
            {t("alfred_ds_step_sub")}
          </p>
        </div>
        {showNewBadge && (
          <span
            data-testid="ds-new-badge"
            className="rounded-full px-2 py-0.5 text-[10px] font-medium"
            style={{ background: "var(--accent-bg)", color: "var(--accent-fg)" }}
          >
            {t("alfred_ds_new_badge")}
          </span>
        )}
      </header>

      {loading && (
        <ul
          data-testid="ds-skeleton"
          className="grid gap-4 sm:grid-cols-2"
          aria-busy="true"
        >
          {SKELETON_KEYS.map((k) => (
            <li
              key={k}
              className="rounded border border-neutral-200 bg-white p-3 dark:border-neutral-800 dark:bg-neutral-900"
            >
              <div className="h-4 w-24 animate-pulse rounded bg-neutral-200 dark:bg-neutral-800" />
              <div className="mt-2 h-3 w-40 animate-pulse rounded bg-neutral-200 dark:bg-neutral-800" />
              <div className="mt-3 grid grid-cols-2 gap-2">
                <div className="aspect-[4/3] animate-pulse rounded bg-neutral-200 dark:bg-neutral-800" />
                <div className="aspect-[4/3] animate-pulse rounded bg-neutral-200 dark:bg-neutral-800" />
              </div>
            </li>
          ))}
        </ul>
      )}

      {showEmptyState && (
        <div
          data-testid="ds-empty-state"
          className="rounded border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200"
        >
          <p>{t("alfred_ds_load_error")}</p>
          <button
            type="button"
            onClick={onSkip}
            className="mt-3 rounded bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900"
          >
            {t("alfred_ds_select_cta")}
          </button>
        </div>
      )}

      {!loading && !showEmptyState && (
        <>
          <ul role="radiogroup" aria-label={t("alfred_ds_step_title")} className="grid gap-4 sm:grid-cols-2">
            {catalog.map((entry) => {
              const ref = refOf(entry);
              const isSelected = ref === selectedRef;
              const labelId = `ds-card-${entry.asset_id}`;
              return (
                <li key={ref}>
                  <button
                    type="button"
                    role="radio"
                    aria-checked={isSelected}
                    aria-labelledby={labelId}
                    data-testid={`ds-card-${entry.asset_id}`}
                    onClick={() => onSelect(ref)}
                    className={
                      "block w-full rounded border bg-white p-3 text-left transition focus:outline-none focus:ring-2 focus:ring-offset-2 dark:bg-neutral-900 " +
                      (isSelected
                        ? "border-neutral-900 ring-1 ring-neutral-900 dark:border-neutral-100 dark:ring-neutral-100"
                        : "border-neutral-200 hover:border-neutral-400 dark:border-neutral-800 dark:hover:border-neutral-600")
                    }
                  >
                    <div
                      id={labelId}
                      className="text-lg"
                      style={{ fontFamily: "var(--font-instrument-serif), serif", fontStyle: "italic" }}
                    >
                      {entry.name}
                    </div>
                    {entry.manifest?.use_case && (
                      <p className="mt-1 text-sm" style={{ color: "var(--fg-2)" }}>
                        {entry.manifest.use_case}
                      </p>
                    )}
                    <div className="mt-3 grid grid-cols-2 gap-2">
                      {entry.manifest?.screenshots?.light && (
                        <figure className="space-y-1">
                          <img
                            src={entry.manifest.screenshots.light}
                            alt=""
                            className="rounded border border-neutral-200 dark:border-neutral-800"
                          />
                          <figcaption className="text-[10px]" style={{ color: "var(--fg-3)" }}>
                            {t("alfred_ds_card_light_label")}
                          </figcaption>
                        </figure>
                      )}
                      {entry.manifest?.screenshots?.dark && (
                        <figure className="space-y-1">
                          <img
                            src={entry.manifest.screenshots.dark}
                            alt=""
                            className="rounded border border-neutral-200 dark:border-neutral-800"
                          />
                          <figcaption className="text-[10px]" style={{ color: "var(--fg-3)" }}>
                            {t("alfred_ds_card_dark_label")}
                          </figcaption>
                        </figure>
                      )}
                    </div>
                  </button>
                </li>
              );
            })}
          </ul>
          <p className="text-xs" style={{ color: "var(--fg-3)" }}>
            {t("alfred_ds_default_hint")}
          </p>
          <div className="flex items-center gap-3">
            <button
              type="button"
              data-testid="ds-continue"
              onClick={onContinue}
              disabled={!selectedRef}
              className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
            >
              {t("alfred_ds_select_cta")}
            </button>
            <button
              type="button"
              data-testid="ds-skip"
              onClick={onSkip}
              className="rounded border border-neutral-300 px-4 py-2 text-sm text-neutral-700 hover:bg-neutral-100 dark:border-neutral-700 dark:text-neutral-200 dark:hover:bg-neutral-800"
            >
              {t("alfred_ds_skip")}
            </button>
          </div>
        </>
      )}
    </section>
  );
}

export default DesignSystemPicker;
