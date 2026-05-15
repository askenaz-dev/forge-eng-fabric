# Alfred Console — User Guide

Alfred is the Forge platform's autonomous operator. The Alfred Console (`/alfred`) is your primary interface for creating, improving and operating Apps.

## Views

The console has two views. The view you see first depends on your role:

| Your role | Default view |
|---|---|
| Product owner, PM, stakeholder | Friendly |
| Engineer, platform operator | Advanced |

You can switch views at any time from the account menu (bottom-left of the sidebar) using the **Modo desarrollador** / **Developer mode** toggle, or by clicking the "Cambiar a modo desarrollador →" link on the Friendly landing page.

## Friendly view

The Friendly view is designed for non-technical stakeholders. It guides you through a short conversation to capture your intent, without requiring slash commands.

### Landing: three cards

When you open `/alfred` in Friendly mode, you see three cards:

| Card | Use it when… |
|---|---|
| **Nueva App** | You want to create a brand new application from scratch |
| **Mejorar** | You want to add a feature, fix a bug, or extend an existing App |
| **Operar** | You want to deploy, monitor, or troubleshoot an existing App |

Click **Empezar** (Start) on any card to open a scoped conversation.

### Conversation panel

After choosing a card, Alfred asks a few short questions to understand your intent. Answer each question in plain language — no commands or technical syntax required.

- Use the **← Volver** (Back) button to return to the card selection.
- If your description matches an existing spec in the platform, a **Match dialog** may appear (see below).

### App switcher

If your workspace has more than one App, an App picker appears. Selecting an App scopes Alfred's conversation to that App's context.

### Label-only rendering

In Friendly view, Alfred never shows raw IDs (UUIDs, spec slugs, workflow IDs). All entity references appear as human-readable labels. If a label cannot be resolved, Alfred shows a placeholder like _a recent App_.

### Error messages

If something goes wrong, Alfred shows a plain-language message. You can click **Ver detalles técnicos** (Show technical details) to see the raw error — useful for sharing with your engineering team.

## Advanced view

The Advanced view is the full slash-command console. It is designed for engineers and platform operators.

### Slash commands

| Command | What it does |
|---|---|
| `/forge new` | Start a new OpenSpec (replaces the deprecated `/openspec new`) |
| `/forge list` | List your OpenSpecs |
| `/forge edit <id>` | Edit an existing OpenSpec |
| free text | Start a new intent directly |

> **Note:** `/openspec` is a deprecated alias and will be removed in two minor versions. Use `/forge` for all new commands.

### App scoping

Use the App picker in the top bar to scope commands to a specific App. Subsequent commands use that App's context.

### Keyboard shortcuts

| Shortcut | Action |
|---|---|
| `/` | Open the command palette |
| `⌘K` / `Ctrl+K` | Open the command palette |
| `Enter` | Submit the current input |
| `Esc` | Close the command palette |

## Match dialog

When Alfred detects that your intent may already be captured in an existing spec, it shows the **Match dialog** before creating a new draft.

### Actions

| Action | When it appears | What it does |
|---|---|---|
| **Implementar** | Match is `approved` or `committed` | Starts an agent-mode session jumping directly to the architect phase |
| **Extender** | Any match | Opens a conversation to extend the existing spec |
| **Crear nuevo** | Always | Bypasses the match and creates a new spec |
| **No, esto no es lo mismo** | Always | Dismisses the match and starts a new spec |

## FAQ

**My role changed — why am I still seeing the old view?**
Your view preference is persisted per-user. To reset it, open the account menu → **Developer mode** toggle, or sign out and sign back in.

**Can I lock all users in my team to Friendly view?**
Yes. Ask your platform admin to set `tenant.console_default_view = "friendly"`. This overrides the role-based default for all new users in your tenant.

**The match dialog appeared but the spec is unrelated — what do I do?**
Click **No, esto no es lo mismo**. Alfred records your feedback and creates a fresh draft. This helps improve future match accuracy.
