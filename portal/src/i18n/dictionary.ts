// Bilingual dictionary for the Forge Portal.
// Default locale is `es`; English mirrors every key.
// Adding a key requires the matching translation in both maps — enforced by
// the parity-check script (`pnpm --filter @forge/portal run i18n:check`) and
// by the typescript `Dictionary` type below.

import type { Lang } from "@/lib/prefs";

const DICTIONARY = {
  es: {
    // Navigation groups
    nav_platform:    "Plataforma",
    nav_govern:      "Gobierno",
    nav_observe:     "Observabilidad",
    nav_account:     "Cuenta",

    // Navigation items
    nav_dashboard:   "Tablero",
    nav_workspaces:  "Workspaces",
    nav_tenants:     "Tenants",
    nav_agents:      "Agentes",
    nav_skills:      "Skills",
    nav_mcp:         "Herramientas MCP",
    nav_gateway:     "Skill gateway",
    nav_workflows:   "Workflows",
    nav_approvals:   "Aprobaciones",
    nav_specs:       "Specs (OpenSpec)",
    nav_policies:    "Políticas (OPA)",
    nav_audit:       "Auditoría",
    nav_obs:         "Métricas y trazas",
    nav_incidents:   "Incidentes",
    nav_deployments: "Despliegues",
    nav_drift:       "Drift",
    nav_evolution:   "Evolución",
    nav_finops:      "FinOps",
    nav_marketplace: "Marketplace",
    nav_templates:   "Plantillas",
    nav_runtimes:    "Runtimes",
    nav_assets:      "Registro de activos",
    nav_initiatives: "Iniciativas",
    nav_alfred:      "Alfred",
    nav_apps_new:    "Nueva App",
    nav_onboarding:  "Onboarding",
    nav_pr_gates:    "Gates de PR",
    nav_kill_switch: "Kill switch",
    nav_permissions: "Permisos",
    nav_settings:    "Ajustes",

    // Top bar
    tb_search:        "Buscar agentes, runs, skills, specs…",
    tb_search_short:  "Buscar…",
    tb_notif:         "Notificaciones",
    tb_theme:         "Tema",
    tb_lang:          "Idioma",
    tb_command_hint:  "para ir a cualquier sitio",

    // Theme
    theme_light:       "Claro",
    theme_dark:        "Oscuro",
    theme_system:      "Sistema",
    theme_system_hint: "Sigue al SO",

    // Density
    density_compact:     "Compacta",
    density_comfortable: "Cómoda",
    density_spacious:    "Amplia",

    // Crumbs
    crumb_workspace: "Workspace · Engineering",

    // Greetings (time-of-day)
    h_hello:    "Buenos días,",
    h_hello_pm: "Buenas tardes,",
    h_hello_n:  "Buenas noches,",

    // Dashboard headline
    h_overview_em:   "telar",
    h_overview_pre:  "Tu",
    h_overview_post: "está corriendo en caliente. {agents} agentes activos, {approvals} aprobaciones esperándote.",

    // CTAs
    h_new_run: "Lanzar workflow",
    h_invite:  "Invitar equipo",
    h_live:    "en vivo · refresca cada 2s",

    // KPIs
    kpi_runs:    "Runs en curso",
    kpi_success: "Éxito 24 h",
    kpi_p95:     "p95 latencia",
    kpi_savings: "Horas ahorradas / semana",
    kpi_vs:      "vs semana pasada",

    // Runs
    runs_title:          "Runs recientes",
    runs_sub:            "últimos 50 · todas las branches",
    runs_filter_all:     "Todos",
    runs_filter_running: "Corriendo",
    runs_filter_succ:    "Exitosos",
    runs_filter_failed:  "Fallidos",
    runs_filter_wait:    "Esperando aprobación",
    runs_view_all:       "Ver todos",
    runs_filters:        "Filtros",

    // Run statuses
    st_running: "Corriendo",
    st_success: "OK",
    st_failed:  "Fallido",
    st_pending: "Aprobación",
    st_queued:  "En cola",

    // Approvals
    apr_title:       "Cola de aprobación",
    apr_sub:         "Human-in-the-loop · OPA",
    apr_approve:     "Aprobar",
    apr_reject:      "Rechazar",
    apr_review:      "Revisar",
    apr_expires_in:  "vence en",
    apr_no_items:    "Sin aprobaciones pendientes.",

    // Activity timeline
    act_title: "Actividad de la plataforma",
    act_sub:   "eventos firmados · audit-log",
    act_view:  "Ver auditoría",

    // Services mesh
    svc_title:    "Mesh de servicios",
    svc_sub:      "Temporal · OPA · OpenFGA · pgvector",
    svc_healthy:  "sano",
    svc_degraded: "degradado",
    svck_orch:     "Orquestación",
    svck_policy:   "Política",
    svck_authz:    "AuthZ",
    svck_registry: "Registro",
    svck_audit:    "Auditoría",
    svck_ctx:      "Contexto",
    svck_spec:     "Specs",
    svck_db:       "Datos",

    // Sheet (run detail)
    sheet_run:             "Run",
    sheet_input:           "Entrada",
    sheet_steps:           "Pasos del workflow",
    sheet_policy_eval:     "Evaluación de política",
    sheet_policy_pass:     "Permitido por",
    sheet_diff_title:      "Diff propuesto",
    sheet_diff_before:     "Antes",
    sheet_diff_after:      "Después",
    sheet_close:           "Cerrar",
    sheet_approve_apply:   "Aprobar y aplicar",
    sheet_request_changes: "Pedir cambios",

    // Toasts
    toast_approved:   "Aprobado. El agente continúa la ejecución.",
    toast_rejected:   "Rechazado. Se notificó al agente.",
    toast_lang:       "Idioma cambiado a Español",
    toast_lang_en:    "Idioma cambiado a Inglés",
    toast_theme:      "Tema actualizado",
    toast_density:    "Densidad actualizada",
    toast_workspace:  "Workspace actualizado",

    // Footer
    foot_role:   "Plataforma · Ingeniero",
    invite_lbl:  "Invitar",

    // Misc shared labels
    every:        "cada",
    secs:         "s",
    avg_today:    "promedio hoy",
    by:           "por",
    in_branch:    "en",
    triggered:    "lanzado por",
    duration:     "duración",
    policy_short: "OPA",
    open:         "Abrir",
    copy_id:      "Copiar ID",
    sign_out:     "Cerrar sesión",
    account_menu: "Cuenta de usuario",
    expand_section:   "Expandir sección",
    collapse_section: "Contraer sección",
    no_data:      "sin datos",
    unavailable:  "no disponible",
    tenant_active:        "Tenant activo",
    tenant_no_others:     "No tienes acceso a otros tenants.",
    tenant_no_workspaces: "Este tenant aún no tiene workspaces.",
    tenant_manage:        "Gestionar tenants",
    tenants_title:        "Tenants",
    tenants_sub:          "Crea tenants nuevos y revisa los existentes. Acción restringida a `platform-admin`.",
    tenants_new:          "Nuevo tenant",
    tenants_name:         "Nombre",
    tenants_create:       "Crear",
    tenants_empty:        "Aún no hay tenants.",
    tenants_loading:      "Cargando tenants…",
    request_access: "Solicitar acceso",
    forbidden:    "No tienes permiso para ver esta sección.",
    min_width_notice: "Mejor experiencia en pantallas ≥ 720px",
    refresh:      "Refrescar",

    // Activity event templates ({name} placeholders)
    ev_run_started:    "El agente {agent} inició un run en {repo}",
    ev_apr_granted:    "Aprobado el cambio sobre {target} por {who}",
    ev_policy_denied:  "Política {policy} bloqueó la herramienta {tool}",
    ev_skill_pub:      "Nueva skill {skill} publicada por {who}",
    ev_self_heal:      "Self-healing reinició {service} tras 3 errores 5xx",
    ev_spec_merged:    "Spec {spec} mergeada · contratos actualizados",

    // Command palette
    cmd_placeholder:    "Saltar a un agente, run, skill, spec…",
    cmd_no_results:     "Sin resultados.",
    cmd_results_count:  "{n} resultados",
    cmd_group_nav:      "Navegación",
    cmd_group_agents:   "Agentes",
    cmd_group_skills:   "Skills",
    cmd_group_mcp:      "Herramientas MCP",
    cmd_group_runs:     "Runs",
    cmd_group_approvals:"Aprobaciones",
    cmd_group_specs:    "Specs",
    cmd_group_workspaces:"Workspaces",
    cmd_group_tenants:  "Tenants",
    cmd_group_actions:  "Acciones",
    cmd_action_theme_light: "Cambiar a tema claro",
    cmd_action_theme_dark:  "Cambiar a tema oscuro",
    cmd_action_theme_system:"Seguir tema del sistema",
    cmd_action_lang_es: "Cambiar a Español",
    cmd_action_lang_en: "Cambiar a Inglés",
    cmd_action_sidebar: "Alternar barra lateral",
    cmd_action_sign_out:"Cerrar sesión",
    cmd_unavailable:    "fuente no disponible",
  },
  en: {
    nav_platform:    "Platform",
    nav_govern:      "Governance",
    nav_observe:     "Observability",
    nav_account:     "Account",

    nav_dashboard:   "Dashboard",
    nav_workspaces:  "Workspaces",
    nav_tenants:     "Tenants",
    nav_agents:      "Agents",
    nav_skills:      "Skills",
    nav_mcp:         "MCP Tools",
    nav_gateway:     "Skill gateway",
    nav_workflows:   "Workflows",
    nav_approvals:   "Approvals",
    nav_specs:       "Specs (OpenSpec)",
    nav_policies:    "Policies (OPA)",
    nav_audit:       "Audit",
    nav_obs:         "Metrics & traces",
    nav_incidents:   "Incidents",
    nav_deployments: "Deployments",
    nav_drift:       "Drift",
    nav_evolution:   "Evolution",
    nav_finops:      "FinOps",
    nav_marketplace: "Marketplace",
    nav_templates:   "Templates",
    nav_runtimes:    "Runtimes",
    nav_assets:      "Asset registry",
    nav_initiatives: "Initiatives",
    nav_alfred:      "Alfred",
    nav_apps_new:    "New App",
    nav_onboarding:  "Onboarding",
    nav_pr_gates:    "PR gates",
    nav_kill_switch: "Kill switch",
    nav_permissions: "Permissions",
    nav_settings:    "Settings",

    tb_search:        "Search agents, runs, skills, specs…",
    tb_search_short:  "Search…",
    tb_notif:         "Notifications",
    tb_theme:         "Theme",
    tb_lang:          "Language",
    tb_command_hint:  "to navigate anywhere",

    theme_light:       "Light",
    theme_dark:        "Dark",
    theme_system:      "System",
    theme_system_hint: "Follow OS",

    density_compact:     "Compact",
    density_comfortable: "Comfortable",
    density_spacious:    "Spacious",

    crumb_workspace: "Workspace · Engineering",

    h_hello:    "Good morning,",
    h_hello_pm: "Good afternoon,",
    h_hello_n:  "Good evening,",

    h_overview_em:   "loom",
    h_overview_pre:  "Your",
    h_overview_post: "is running hot. {agents} agents active, {approvals} approvals waiting on you.",

    h_new_run: "Launch workflow",
    h_invite:  "Invite team",
    h_live:    "live · refreshes every 2s",

    kpi_runs:    "Runs in flight",
    kpi_success: "Success 24 h",
    kpi_p95:     "p95 latency",
    kpi_savings: "Hours saved / week",
    kpi_vs:      "vs last week",

    runs_title:          "Recent runs",
    runs_sub:            "last 50 · all branches",
    runs_filter_all:     "All",
    runs_filter_running: "Running",
    runs_filter_succ:    "Succeeded",
    runs_filter_failed:  "Failed",
    runs_filter_wait:    "Waiting on approval",
    runs_view_all:       "View all",
    runs_filters:        "Filters",

    st_running: "Running",
    st_success: "OK",
    st_failed:  "Failed",
    st_pending: "Approval",
    st_queued:  "Queued",

    apr_title:       "Approval queue",
    apr_sub:         "Human-in-the-loop · OPA",
    apr_approve:     "Approve",
    apr_reject:      "Reject",
    apr_review:      "Review",
    apr_expires_in:  "expires in",
    apr_no_items:    "No pending approvals.",

    act_title: "Platform activity",
    act_sub:   "signed events · audit-log",
    act_view:  "View audit",

    svc_title:    "Service mesh",
    svc_sub:      "Temporal · OPA · OpenFGA · pgvector",
    svc_healthy:  "healthy",
    svc_degraded: "degraded",
    svck_orch:     "Orchestration",
    svck_policy:   "Policy",
    svck_authz:    "AuthZ",
    svck_registry: "Registry",
    svck_audit:    "Audit",
    svck_ctx:      "Context",
    svck_spec:     "Specs",
    svck_db:       "Data",

    sheet_run:             "Run",
    sheet_input:           "Input",
    sheet_steps:           "Workflow steps",
    sheet_policy_eval:     "Policy evaluation",
    sheet_policy_pass:     "Allowed by",
    sheet_diff_title:      "Proposed diff",
    sheet_diff_before:     "Before",
    sheet_diff_after:      "After",
    sheet_close:           "Close",
    sheet_approve_apply:   "Approve & apply",
    sheet_request_changes: "Request changes",

    toast_approved:   "Approved. Agent will resume execution.",
    toast_rejected:   "Rejected. Agent has been notified.",
    toast_lang:       "Language switched to Spanish",
    toast_lang_en:    "Language switched to English",
    toast_theme:      "Theme updated",
    toast_density:    "Density updated",
    toast_workspace:  "Workspace updated",

    foot_role:   "Platform · Engineer",
    invite_lbl:  "Invite",

    every:        "every",
    secs:         "s",
    avg_today:    "today's avg",
    by:           "by",
    in_branch:    "on",
    triggered:    "triggered by",
    duration:     "duration",
    policy_short: "OPA",
    open:         "Open",
    copy_id:      "Copy ID",
    sign_out:     "Sign out",
    account_menu: "User account",
    expand_section:   "Expand section",
    collapse_section: "Collapse section",
    no_data:      "no data",
    unavailable:  "unavailable",
    tenant_active:        "Active tenant",
    tenant_no_others:     "No other tenants available to you.",
    tenant_no_workspaces: "This tenant has no workspaces yet.",
    tenant_manage:        "Manage tenants",
    tenants_title:        "Tenants",
    tenants_sub:          "Create new tenants and review existing ones. Restricted to `platform-admin`.",
    tenants_new:          "New tenant",
    tenants_name:         "Name",
    tenants_create:       "Create",
    tenants_empty:        "No tenants yet.",
    tenants_loading:      "Loading tenants…",
    request_access: "Request access",
    forbidden:    "You don't have permission to view this section.",
    min_width_notice: "Best experience on screens ≥ 720px",
    refresh:      "Refresh",

    ev_run_started:    "Agent {agent} started a run on {repo}",
    ev_apr_granted:    "Approved change on {target} by {who}",
    ev_policy_denied:  "Policy {policy} blocked tool {tool}",
    ev_skill_pub:      "New skill {skill} published by {who}",
    ev_self_heal:      "Self-healing restarted {service} after 3 × 5xx",
    ev_spec_merged:    "Spec {spec} merged · contracts updated",

    cmd_placeholder:    "Jump to an agent, run, skill, spec…",
    cmd_no_results:     "No results.",
    cmd_results_count:  "{n} results",
    cmd_group_nav:      "Navigation",
    cmd_group_agents:   "Agents",
    cmd_group_skills:   "Skills",
    cmd_group_mcp:      "MCP Tools",
    cmd_group_runs:     "Runs",
    cmd_group_approvals:"Approvals",
    cmd_group_specs:    "Specs",
    cmd_group_workspaces:"Workspaces",
    cmd_group_tenants:  "Tenants",
    cmd_group_actions:  "Actions",
    cmd_action_theme_light: "Switch to light theme",
    cmd_action_theme_dark:  "Switch to dark theme",
    cmd_action_theme_system:"Follow system theme",
    cmd_action_lang_es: "Switch to Spanish",
    cmd_action_lang_en: "Switch to English",
    cmd_action_sidebar: "Toggle sidebar",
    cmd_action_sign_out:"Sign out",
    cmd_unavailable:    "source unavailable",
  },
} as const;

// Derive the key set from the ES dictionary (canonical) so the type system
// rejects calls to `t()` with unknown keys.
export type Dictionary = typeof DICTIONARY;
export type DictKey = keyof Dictionary["es"];

export function getDictionary(lang: Lang): Record<DictKey, string> {
  return DICTIONARY[lang] as unknown as Record<DictKey, string>;
}

// Variable substitution: replaces {placeholder} with vars[placeholder].
export function tFmt(template: string, vars?: Record<string, string | number>): string {
  if (!vars) return template;
  return template.replace(/\{(\w+)\}/g, (_, k) => {
    const v = vars[k];
    return v == null ? `{${k}}` : String(v);
  });
}

export { DICTIONARY };
