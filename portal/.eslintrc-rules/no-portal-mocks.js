"use strict";

/**
 * forge/no-portal-mocks
 *
 * Enforces the "real data only" policy for the Portal rebrand. Rejects:
 *   - identifiers matching /^(mock|fixture|fake)_/i
 *   - imports from the design/ folder (brand notebook fixtures)
 *   - inline array literals with > 3 object elements that all carry `id` AND
 *     `title` keys (heuristic match for a fixture array shape)
 *
 * Whitelist via the inline comment `// portal-mock-ok:<reason>`.
 */

const FORBIDDEN_IDENT = /^(mock|fixture|fake)_/i;
const DESIGN_IMPORT = /(^|\/)design\//;

/** @type {import('eslint').Rule.RuleModule} */
module.exports = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Forbid mock / fixture identifiers, design/ imports, and inline fixture arrays in portal/src",
      recommended: true,
    },
    schema: [],
    messages: {
      forbiddenIdent:
        "Identifier '{{name}}' looks like a mock or fixture. The Portal must consume real platform APIs only.",
      forbiddenImport:
        "Importing from 'design/' is not allowed in shipped Portal source. The notebook fixtures must not ship.",
      fixtureArray:
        "Inline array of {{count}} fixture-shaped objects detected (id + title keys). Bind to a real API instead.",
    },
  },
  create(context) {
    const filename = context.getFilename();
    if (!filename.includes(`${require("path").sep}portal${require("path").sep}src${require("path").sep}`)) {
      return {};
    }

    function hasOptOut(node) {
      const comments = context.getSourceCode().getCommentsBefore(node);
      return comments.some((c) => /portal-mock-ok:/i.test(c.value));
    }

    return {
      Identifier(node) {
        if (FORBIDDEN_IDENT.test(node.name) && !hasOptOut(node)) {
          context.report({ node, messageId: "forbiddenIdent", data: { name: node.name } });
        }
      },
      ImportDeclaration(node) {
        const source = String(node.source.value || "");
        if (DESIGN_IMPORT.test(source) && !hasOptOut(node)) {
          context.report({ node, messageId: "forbiddenImport" });
        }
      },
      ArrayExpression(node) {
        if (node.elements.length <= 3) return;
        const objects = node.elements.filter((el) => el && el.type === "ObjectExpression");
        if (objects.length !== node.elements.length) return;
        const allHaveIdTitle = objects.every((obj) =>
          obj.properties.some((p) => p.type === "Property" && p.key && p.key.name === "id") &&
          obj.properties.some((p) => p.type === "Property" && p.key && p.key.name === "title"),
        );
        if (allHaveIdTitle && !hasOptOut(node)) {
          context.report({
            node,
            messageId: "fixtureArray",
            data: { count: String(node.elements.length) },
          });
        }
      },
    };
  },
};
