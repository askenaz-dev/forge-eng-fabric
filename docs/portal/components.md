# Forge Portal — Component reference

This is the developer-facing inventory of the design-system primitives. Every
component listed lives under `portal/src/components/primitives/` (or its
neighbouring `shell/`, `runs/`, `approvals/`, `dashboard/` directories) and
exports named props.

## Primitives

### `<Button />`

Variants: `primary`, `secondary` (default), `ghost`, `danger`.
Sizes:    `default`, `xs`.

```tsx
import { Button } from "@/components/primitives";
import { Plus } from "@/components/icons";

<Button variant="primary" leading={<Plus />}>Lanzar workflow</Button>
```

### `<Badge />`

Tones: `default`, `ok`, `warn`, `err`, `ember`, `info`, `steel`.
Optional `dot` boolean adds a leading status dot.

```tsx
<Badge tone="ok" dot>healthy</Badge>
```

### `<Card />` + `<CardHeader />` + `<CardBody />`

```tsx
<Card>
  <CardHeader title="Runs recientes" sub="últimos 50" right={<Button variant="ghost" size="xs">Refrescar</Button>} />
  <CardBody>…</CardBody>
</Card>
```

### `<Kpi />`

```tsx
<Kpi
  label={t("kpi_p95")}
  icon={Clock}
  num="184"
  unit="ms"
  delta={{ dir: "down", v: "−18ms" }}
  foot={t("avg_today")}
  data={[220, 212, 205, 200, 196, 210, 202, 194, 188, 190, 186, 184]}
  color="var(--info)"
/>
```

### `<Chip />` + `<ChipRow />`

```tsx
<ChipRow>
  <Chip pressed={filter === "all"} onClick={() => setFilter("all")} count={50}>Todos</Chip>
  <Chip pressed={filter === "running"} onClick={() => setFilter("running")} count={12}>Corriendo</Chip>
</ChipRow>
```

### `<Seg />`

Segmented control. `value`, `options`, `onChange` are required.

```tsx
<Seg
  value={view}
  options={[{ value: "list", label: "Lista" }, { value: "graph", label: "Grafo" }]}
  onChange={setView}
/>
```

### `<Sheet />`

Right-side panel built on `@radix-ui/react-dialog` with focus trap and
`Escape` close.

```tsx
<Sheet
  open={runId != null}
  onOpenChange={(o) => (o ? null : closeRun())}
  title={<><em>Run</em> · Deploy v1.42.0 → prod</>}
  subtitle="wf_8a13c1 · deploy-conductor"
>
  …
</Sheet>
```

### `<PulseDot />`

```tsx
<PulseDot tone="ok" />     // thread pulse
<PulseDot tone="warn" />   // spark
<PulseDot tone="err" />    // rust
<PulseDot tone="pending" />// info
<PulseDot tone="queued" /> // muted
```

### `<Spark />`

Inline SVG sparkline used by `<Kpi />`.

```tsx
<Spark data={[3, 5, 4, 6, 8, 7, 9, 11, 10, 13, 12, 14]} color="var(--primary)" />
```

### `<Terminal />` + `<Code />`

```tsx
<Terminal title="forge run">
  <span className="prompt">$</span> forge run review.deep --pr 4821
</Terminal>

<Code>{JSON.stringify(payload, null, 2)}</Code>
```

### `<ToastRail />`

Mounted globally by `PortalShell`. Consume via `useToast()`:

```tsx
const toast = useToast();
toast.success(t("toast_theme"));
toast.err("Operation failed");
```

## Shell

- `<PortalShell />` — composes `Sidebar` + `TopBar` + `<main>` + `ToastRail` +
  `CommandPalette`. Mounted by `layout.tsx`.
- `<Sidebar />` — branded nav, tenant pill, footer avatar.
- `<TopBar />` — breadcrumb, search trigger, lang/theme/notifications/github.
- `<ThemeMenu />` — Radix DropdownMenu with three theme options.
- `<LangPill />` — ES / EN toggle.
- `<NotificationsButton />` — Bell + SSE-driven dot + popover.
- `<CommandPalette />` — `cmdk` + Radix Dialog with sources.

## Page conventions

Use `<PageHead />` at the top of every page to render the eyebrow + serif
italic-accent title + sub + actions row.

```tsx
import { PageHead } from "@/components/page/PageHead";

<PageHead
  eyebrow="Governance · Approvals"
  title="Cola de"
  titleEm="aprobación"
  sub="Pausas de política con contexto de intención…"
  actions={<Button variant="primary">Filtrar</Button>}
/>
```
