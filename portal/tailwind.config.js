/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./src/app/**/*.{ts,tsx}",
    "./src/components/**/*.{ts,tsx}",
    "./src/lib/**/*.{ts,tsx}",
  ],
  darkMode: ["class", '[data-theme="dark"]'],
  theme: {
    extend: {
      colors: {
        // Brand ramp
        "ember-50":  "var(--ember-50)",
        "ember-100": "var(--ember-100)",
        "ember-200": "var(--ember-200)",
        "ember-300": "var(--ember-300)",
        "ember-400": "var(--ember-400)",
        "ember-500": "var(--ember-500)",
        "ember-600": "var(--ember-600)",
        "ember-700": "var(--ember-700)",
        "ember-800": "var(--ember-800)",
        "ember-900": "var(--ember-900)",

        // Semantic tones
        thread: "var(--thread)",
        spark:  "var(--spark)",
        rust:   "var(--rust)",
        steel:  "var(--steel)",
        copper: "var(--copper)",
        info:   "var(--info)",

        // Primary
        primary:   "var(--primary)",
        "primary-h": "var(--primary-h)",
        "on-primary": "var(--on-primary)",
        "fg-on-ember": "var(--fg-on-ember)",

        // Surfaces
        bg:        "var(--bg)",
        "bg-2":    "var(--bg-2)",
        "bg-app":  "var(--bg-app)",
        "bg-side": "var(--bg-side)",
        "bg-canvas": "var(--bg-canvas)",
        "bg-card": "var(--bg-card)",
        "bg-elev": "var(--bg-elev)",
        "bg-input": "var(--bg-input)",
        "bg-hover": "var(--bg-hover)",
        "bg-active": "var(--bg-active)",
        "bg-sunk":  "var(--bg-sunk)",

        // Foreground
        fg:     "var(--fg)",
        "fg-2": "var(--fg-2)",
        "fg-3": "var(--fg-3)",

        // Borders
        border:   "var(--border)",
        "border-2": "var(--border-2)",
        "border-strong": "var(--border-strong)",

        // Ink + paper anchors
        ink:   "var(--ink)",
        paper: "var(--paper)",
      },
      fontFamily: {
        display: ["var(--f-display)"],
        sans:    ["var(--f-sans)"],
        mono:    ["var(--f-mono)"],
      },
      borderRadius: {
        "r-1": "var(--r-1)",
        "r-2": "var(--r-2)",
        "r-3": "var(--r-3)",
        "r-4": "var(--r-4)",
        "r-5": "var(--r-5)",
        "r-pill": "var(--r-pill)",
      },
      boxShadow: {
        "shadow-1":   "var(--shadow-1)",
        "shadow-2":   "var(--shadow-2)",
        "shadow-pop": "var(--shadow-pop)",
        "shadow-emb": "var(--shadow-emb)",
        ring:         "var(--ring)",
      },
      transitionTimingFunction: {
        fast: "cubic-bezier(.2,.6,.3,1)",
        med:  "cubic-bezier(.2,.6,.3,1)",
      },
      transitionDuration: {
        fast: "120ms",
        med:  "220ms",
      },
      spacing: {
        "s-1":  "var(--s-1)",
        "s-2":  "var(--s-2)",
        "s-3":  "var(--s-3)",
        "s-4":  "var(--s-4)",
        "s-5":  "var(--s-5)",
        "s-6":  "var(--s-6)",
        "s-8":  "var(--s-8)",
        "s-10": "var(--s-10)",
        "s-12": "var(--s-12)",
        "s-16": "var(--s-16)",
        "s-20": "var(--s-20)",
        "s-24": "var(--s-24)",
        sidebar: "var(--sidebar-w)",
        topbar:  "var(--topbar-h)",
      },
    },
  },
  plugins: [],
};
