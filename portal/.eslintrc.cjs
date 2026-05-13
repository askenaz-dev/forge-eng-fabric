"use strict";

// Portal ESLint config.
//
// The custom `no-portal-mocks` heuristic rule lives at `.eslintrc-rules/`
// for reference and `next lint --rulesdir ./.eslintrc-rules` usage, but it
// is *not* registered here because `next build` runs ESLint without the
// `--rulesdir` flag and cannot resolve in-repo rule definitions.
// The canonical enforcement of the "real data only" policy is
// `scripts/audit-no-mocks.sh`, wired in CI via `.github/workflows/portal-lint.yml`.

module.exports = {
  root: true,
  extends: ["next/core-web-vitals"],
};
