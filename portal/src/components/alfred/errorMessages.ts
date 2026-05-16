/**
 * Maps known Alfred/OpenFGA backend error codes to friendly copy keys.
 * (alfred-console-redesign requirement 1.6)
 *
 * The lookup returns the i18n key; unknown codes fall back to `alfred_err_generic`.
 * The raw payload is always preserved so the disclosure toggle can show it.
 */
export type FriendlyErrorKey =
  | "alfred_err_missing_app_editor"
  | "alfred_err_feature_disabled"
  | "alfred_err_generic";

const ERROR_MAP: Record<string, FriendlyErrorKey> = {
  missing_app_editor: "alfred_err_missing_app_editor",
  "403 missing_app_editor": "alfred_err_missing_app_editor",
  forbidden: "alfred_err_missing_app_editor",
  missing_app_scope: "alfred_err_generic",
  workspace_can_edit_required: "alfred_err_generic",
  "dialogue api disabled": "alfred_err_feature_disabled",
};

export function mapErrorCode(rawDetail: string): FriendlyErrorKey {
  const normalised = rawDetail.toLowerCase().trim();
  for (const [pattern, key] of Object.entries(ERROR_MAP)) {
    if (normalised.includes(pattern.toLowerCase())) return key;
  }
  return "alfred_err_generic";
}
