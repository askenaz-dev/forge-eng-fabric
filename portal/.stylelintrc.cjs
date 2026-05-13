"use strict";

// Stylelint config — enforces the Forge design-system contract:
// raw hex/rgb/hsl colours are forbidden everywhere except globals.css
// (the single sheet that declares the token surface).

module.exports = {
  extends: ["stylelint-config-standard"],
  plugins: ["stylelint-declaration-strict-value"],
  rules: {
    "scale-unlimited/declaration-strict-value": [
      ["/color/", "background-color", "border-color", "fill", "stroke"],
      {
        ignoreValues: [
          "currentColor",
          "transparent",
          "inherit",
          "initial",
          "unset",
          "/^var\\(/",
          "/^color-mix\\(/",
          "/^url\\(/",
          "/^linear-gradient\\(/",
          "/^radial-gradient\\(/",
          "/^conic-gradient\\(/",
          "/^oklch\\(/",
          "/^lab\\(/",
        ],
        severity: "error",
      },
    ],
  },
  ignoreFiles: ["src/app/globals.css", "**/*.module.css", "node_modules/**", ".next/**"],
};
