"use strict";

// In-repo ESLint plugin "forge".
// Exposes our custom rules so they can be referenced as `forge/<rule-id>`
// in eslintConfig.rules.

module.exports = {
  rules: {
    "no-portal-mocks": require("./no-portal-mocks.js"),
  },
};
