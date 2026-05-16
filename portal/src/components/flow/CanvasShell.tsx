"use client";

// CanvasShell wires the React Flow canvas together with the palette,
// property panel, code-view tab, and dry-run drawer. Persists changes
// back to workflow-registry through the editor page's save action.
//
// The AST round-trip (canvas state ↔ canonical YAML/JSON) goes through
// `ast-canvas-adapter`. Drag-drop, edge-routing, zoom, pan, minimap
// come from @xyflow/react.

import { useCallback, useMemo, useRef, useState, useEffect } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  addEdge,
  useEdgesState,
  useNodesState,
  type Connection,
  type Edge,
  type Node,
  type NodeMouseHandler,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import {
  astToCanvas,
  canvasToAST,
  type CanonicalStep,
  type CanonicalTrigger,
  type CanonicalWorkflow,
  type CanonicalStepType,
  type CanonicalTriggerType,
} from "@/lib/ast-canvas-adapter";
import { FlowNode, type FlowNodeData } from "./nodes/FlowNode";
import { Palette, type PaletteItem } from "./Palette";
import { PropertyPanel, type PropertyPanelCatalogs, type SelectedNode } from "./PropertyPanel";
import { CodeViewTab } from "./CodeViewTab";
import { DryRunDrawer } from "./DryRunDrawer";
import type { DryRunStepTrace } from "./types";

const nodeTypes = { default: FlowNode };

export interface CanvasShellProps {
  workspaceId: string;
  workflowId?: string;
  initialAst: CanonicalWorkflow;
  readOnly?: boolean;
  catalogs?: PropertyPanelCatalogs;
  onSave?: (ast: CanonicalWorkflow) => Promise<void>;
  onDryRun?: (ast: CanonicalWorkflow) => Promise<DryRunStepTrace[]>;
}

export default function CanvasShell(props: CanvasShellProps) {
  const initialGraph = useMemo(() => astToCanvas(props.initialAst), [props.initialAst]);
  const [nodes, setNodes, onNodesChange] = useNodesState<Node<FlowNodeData>>(
    initialGraph.nodes.map((n, idx) => ({
      id: n.id,
      type: "default",
      position: n.position ?? { x: 100 + (idx % 4) * 220, y: 100 + Math.floor(idx / 4) * 160 },
      data: {
        kind: "step",
        type: (n.data.nodeType as CanonicalStepType),
        label: n.data.label ?? n.id,
        ref: n.data.ref,
        tool: n.data.tool,
      },
    })),
  );
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>(
    initialGraph.edges.map((e) => ({ id: e.id, source: e.source, target: e.target })),
  );

  // Triggers live alongside steps but render in a dedicated visual band.
  const [triggers, setTriggers] = useState<CanonicalTrigger[]>(props.initialAst.spec.triggers ?? []);

  const [tab, setTab] = useState<"canvas" | "code">("canvas");
  const [codeViewText, setCodeViewText] = useState<string>(() => JSON.stringify(props.initialAst, null, 2));
  const [codeViewError, setCodeViewError] = useState<string | null>(null);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerSteps, setDrawerSteps] = useState<DryRunStepTrace[]>([]);
  const [drawerError, setDrawerError] = useState<string | undefined>(undefined);
  const [drawerStartedAt, setDrawerStartedAt] = useState<string | undefined>(undefined);

  const [selected, setSelected] = useState<SelectedNode | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const reactFlowWrapper = useRef<HTMLDivElement>(null);

  const onConnect = useCallback(
    (params: Connection) =>
      setEdges((eds) => addEdge({ id: `${params.source}-${params.target}`, ...params }, eds)),
    [setEdges],
  );

  const buildCurrentAst = useCallback((): CanonicalWorkflow => {
    const graph = {
      nodes: nodes.map((n) => ({
        id: n.id,
        position: n.position,
        type: "default",
        data: {
          label: (n.data as FlowNodeData).label,
          nodeType: (n.data as FlowNodeData).type as CanonicalStepType,
          ref: (n.data as FlowNodeData).ref,
          tool: (n.data as FlowNodeData).tool,
        },
      })),
      edges: edges.map((e) => ({ id: e.id, source: e.source, target: e.target })),
      meta: {
        apiVersion: props.initialAst.apiVersion,
        kind: props.initialAst.kind,
        metadata: props.initialAst.metadata,
        inputs: props.initialAst.spec.inputs ?? [],
        triggers,
        on_failure: props.initialAst.spec.on_failure ?? [],
        outputs: props.initialAst.spec.outputs ?? [],
      },
    };
    return canvasToAST(graph);
  }, [nodes, edges, triggers, props.initialAst]);

  // Keep the code view in sync when the canvas changes.
  useEffect(() => {
    if (tab === "code") return;
    setCodeViewText(JSON.stringify(buildCurrentAst(), null, 2));
  }, [nodes, edges, triggers, tab, buildCurrentAst]);

  const addFromPalette = useCallback(
    (item: PaletteItem, position?: { x: number; y: number }) => {
      const id = nextID(item.type, [...nodes.map((n) => n.id), ...triggers.map((t) => t.id)]);
      if (item.kind === "trigger") {
        setTriggers((prev) => [
          ...prev,
          { id, type: item.type as CanonicalTriggerType, config: {}, outputs: {}, concurrency: "queue" },
        ]);
        return;
      }
      const pos = position ?? { x: 200 + nodes.length * 40, y: 100 + nodes.length * 40 };
      setNodes((prev) => [
        ...prev,
        {
          id,
          type: "default",
          position: pos,
          data: { kind: "step", type: item.type as CanonicalStepType, label: id },
        },
      ]);
    },
    [nodes, triggers, setNodes],
  );

  const onDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      const raw = e.dataTransfer.getData("application/forge-flow-item");
      if (!raw) return;
      let item: PaletteItem;
      try {
        item = JSON.parse(raw) as PaletteItem;
      } catch {
        return;
      }
      const rect = reactFlowWrapper.current?.getBoundingClientRect();
      const position = rect
        ? { x: e.clientX - rect.left - 80, y: e.clientY - rect.top - 20 }
        : undefined;
      addFromPalette(item, position);
    },
    [addFromPalette],
  );

  const onDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
  }, []);

  const onNodeClick: NodeMouseHandler = useCallback((_evt, node) => {
    const d = node.data as FlowNodeData;
    if (d.kind === "trigger") {
      const tr = triggers.find((t) => t.id === node.id);
      if (tr) setSelected({ kind: "trigger", trigger: tr, data: d });
    } else {
      const ast = buildCurrentAst();
      const step = ast.spec.steps.find((s) => s.id === node.id);
      if (step) setSelected({ kind: "step", step, data: d });
    }
  }, [triggers, buildCurrentAst]);

  const onChangeStep = useCallback((id: string, patch: Partial<CanonicalStep>) => {
    setNodes((prev) =>
      prev.map((n) => {
        if (n.id !== id) return n;
        const data = n.data as FlowNodeData;
        return {
          ...n,
          data: {
            ...data,
            ref: patch.ref ?? data.ref,
            tool: patch.tool ?? data.tool,
          },
        };
      }),
    );
    // Update selection so the form re-renders with new values.
    setSelected((prev) =>
      prev && prev.kind === "step" && prev.step.id === id
        ? { ...prev, step: { ...prev.step, ...patch } }
        : prev,
    );
  }, [setNodes]);

  const onChangeTrigger = useCallback((id: string, patch: Partial<CanonicalTrigger>) => {
    setTriggers((prev) => prev.map((t) => (t.id === id ? { ...t, ...patch } : t)));
    setSelected((prev) =>
      prev && prev.kind === "trigger" && prev.trigger.id === id
        ? { ...prev, trigger: { ...prev.trigger, ...patch } }
        : prev,
    );
  }, []);

  async function handleSave() {
    if (!props.onSave) return;
    setSaving(true);
    setSaveError(null);
    try {
      await props.onSave(buildCurrentAst());
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleDryRun() {
    if (!props.onDryRun) return;
    setDrawerOpen(true);
    setDrawerError(undefined);
    setDrawerStartedAt(new Date().toISOString());
    setDrawerSteps([]);
    try {
      const steps = await props.onDryRun(buildCurrentAst());
      setDrawerSteps(steps);
    } catch (err) {
      setDrawerError(err instanceof Error ? err.message : String(err));
    }
  }

  return (
    <div className="flex h-[80vh] relative" data-testid="ai-flow-canvas">
      <Palette
        customNodesRegistered={false}
        onAdd={(item) => addFromPalette(item)}
      />
      <div className="flex-1 relative" ref={reactFlowWrapper}>
        <div className="absolute top-0 left-0 right-0 z-10 flex items-center gap-2 px-3 py-2 bg-white/80 dark:bg-neutral-950/80 backdrop-blur border-b border-neutral-200 dark:border-neutral-800">
          <button
            type="button"
            onClick={() => setTab("canvas")}
            className={`text-xs px-2 py-1 rounded ${tab === "canvas" ? "bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900" : "opacity-70"}`}
            aria-pressed={tab === "canvas"}
          >
            Canvas
          </button>
          <button
            type="button"
            onClick={() => setTab("code")}
            className={`text-xs px-2 py-1 rounded ${tab === "code" ? "bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900" : "opacity-70"}`}
            aria-pressed={tab === "code"}
          >
            Code view
          </button>
          <div className="flex-1" />
          {triggers.length > 0 && (
            <span className="text-[10px] uppercase tracking-wider opacity-60" data-testid="trigger-band">
              Triggered by: {triggers.map((t) => `${t.type}/${t.id}`).join(", ")}
            </span>
          )}
          {triggers.length === 0 && (
            <span className="text-[10px] uppercase tracking-wider opacity-60">
              Triggered by: Manual invoke
            </span>
          )}
          <button
            type="button"
            onClick={handleDryRun}
            className="text-xs px-2 py-1 rounded border border-neutral-300 dark:border-neutral-700"
            disabled={!props.onDryRun || props.readOnly}
          >
            Dry run
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={saving || props.readOnly || codeViewError !== null}
            className="text-xs px-2 py-1 rounded bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900 disabled:opacity-50"
          >
            {saving ? "Saving…" : "Save"}
          </button>
        </div>
        {saveError && (
          <p
            role="alert"
            className="absolute top-12 left-3 right-3 z-10 rounded border border-rose-300 bg-rose-50 p-2 text-xs text-rose-800 dark:border-rose-700 dark:bg-rose-950 dark:text-rose-200"
          >
            {saveError}
          </p>
        )}
        <div className="absolute inset-0 pt-10" onDrop={onDrop} onDragOver={onDragOver}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onNodeClick={onNodeClick}
            nodeTypes={nodeTypes}
            fitView
          >
            <Background />
            <Controls />
            <MiniMap pannable zoomable />
          </ReactFlow>
        </div>
        {tab === "code" && (
          <CodeViewTab
            value={codeViewText}
            onChange={(text, parsed, err) => {
              setCodeViewText(text);
              setCodeViewError(err);
              if (parsed) {
                // Don't re-derive nodes/edges automatically — save triggers it.
              }
            }}
          />
        )}
        <DryRunDrawer
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          steps={drawerSteps}
          startedAt={drawerStartedAt}
          error={drawerError}
        />
      </div>
      <PropertyPanel
        selected={selected}
        onChangeStep={onChangeStep}
        onChangeTrigger={onChangeTrigger}
        catalogs={props.catalogs}
      />
    </div>
  );
}

function nextID(typeHint: string, existing: string[]): string {
  const seen = new Set(existing);
  for (let i = 1; i < 1000; i++) {
    const id = `${typeHint}-${String(i).padStart(2, "0")}`;
    if (!seen.has(id)) return id;
  }
  return `${typeHint}-${Date.now()}`;
}
