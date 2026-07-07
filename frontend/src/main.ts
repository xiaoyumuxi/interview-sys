import "./styles.css";
import {
  ApiClient,
  ApiError,
  type ApiState,
  type CodingQuestion,
  type CodingSubmission,
  type EvaluationCase,
  type EvaluationRun,
  type InterviewSession,
  type JsonObject,
  type Skill,
  type User
} from "./api";
import { locales, normalizeLocale, translate, translateHtml, type Locale } from "./i18n";
import {
  Bot,
  BrainCircuit,
  Captions,
  Check,
  ClipboardCheck,
  Clock3,
  Code2,
  Database,
  FileSearch,
  FileText,
  Flag,
  LayoutDashboard,
  LogIn,
  LogOut,
  Mic,
  MicOff,
  MessagesSquare,
  MonitorUp,
  PanelRightOpen,
  PhoneOff,
  Play,
  Plus,
  Radio,
  RefreshCw,
  RotateCw,
  Save,
  Send,
  Settings2,
  Terminal,
  UserRound,
  Users,
  Video,
  VideoOff,
  X,
  createIcons
} from "lucide";

type View = "dashboard" | "interview" | "coding" | "memory" | "admin" | "evaluation";
type LoadState<T> = { loading: boolean; error: string; data: T };
type RoomPanel = "briefing" | "participants" | "notes";

interface EmptyAction {
  label: string;
  iconName: string;
  action?: string;
  view?: View;
}

interface AppState {
  view: View;
  locale: Locale;
  user: User | null;
  accessToken: string;
  refreshToken: string;
  toast: string;
  skills: Skill[];
  dashboard: LoadState<DashboardData>;
  interview: {
    session: InterviewSession | null;
    trace: JsonObject[];
    report: JsonObject | null;
    answer: string;
    dryRun: boolean;
    error: string;
    loading: boolean;
  };
  meeting: {
    micOn: boolean;
    cameraOn: boolean;
    captionsOn: boolean;
    promptShared: boolean;
    panel: RoomPanel;
    notes: string;
  };
  coding: {
    questions: CodingQuestion[];
    selectedQuestion: CodingQuestion | null;
    submissions: CodingSubmission[];
    language: string;
    sourceCode: string;
    loading: boolean;
    error: string;
  };
  memory: LoadState<JsonObject>;
  admin: LoadState<AdminData>;
  evaluation: {
    cases: EvaluationCase[];
    runs: EvaluationRun[];
    selectedCaseId: string;
    lastRun: JsonObject | null;
    loading: boolean;
    error: string;
  };
}

interface DashboardData {
  health: JsonObject | null;
  worker: JsonObject | null;
  judge: JsonObject | null;
  evalRuns: EvaluationRun[];
  submissions: CodingSubmission[];
}

interface AdminData {
  providers: JsonObject[];
  routes: JsonObject[];
  worker: JsonObject | null;
  judge: JsonObject | null;
}

const api = new ApiClient();
const iconSet = {
  Bot,
  BrainCircuit,
  Captions,
  Check,
  ClipboardCheck,
  Clock3,
  Code2,
  Database,
  FileSearch,
  FileText,
  Flag,
  LayoutDashboard,
  LogIn,
  LogOut,
  Mic,
  MicOff,
  MessagesSquare,
  MonitorUp,
  PanelRightOpen,
  PhoneOff,
  Play,
  Plus,
  Radio,
  RefreshCw,
  RotateCw,
  Save,
  Send,
  Settings2,
  Terminal,
  UserRound,
  Users,
  Video,
  VideoOff,
  X
};
const root = document.querySelector<HTMLDivElement>("#app");

if (!root) {
  throw new Error("missing #app");
}
const appRoot = root;

const state: AppState = {
  view: (localStorage.getItem("frontend:view") as View | null) ?? "dashboard",
  locale: normalizeLocale(localStorage.getItem("frontend:locale") ?? navigator.language),
  user: null,
  accessToken: localStorage.getItem("frontend:access_token") ?? "",
  refreshToken: localStorage.getItem("frontend:refresh_token") ?? "",
  toast: "",
  skills: [],
  dashboard: { loading: false, error: "", data: { health: null, worker: null, judge: null, evalRuns: [], submissions: [] } },
  interview: { session: null, trace: [], report: null, answer: "", dryRun: true, error: "", loading: false },
  meeting: {
    micOn: localStorage.getItem("frontend:meeting:mic") === "on",
    cameraOn: localStorage.getItem("frontend:meeting:camera") === "on",
    captionsOn: localStorage.getItem("frontend:meeting:captions") !== "off",
    promptShared: localStorage.getItem("frontend:meeting:prompt_shared") === "on",
    panel: normalizeRoomPanel(localStorage.getItem("frontend:meeting:panel")),
    notes: localStorage.getItem("frontend:meeting:notes") ?? ""
  },
  coding: {
    questions: [],
    selectedQuestion: null,
    submissions: [],
    language: "go",
    sourceCode: defaultGoSolution(),
    loading: false,
    error: ""
  },
  memory: { loading: false, error: "", data: {} },
  admin: { loading: false, error: "", data: { providers: [], routes: [], worker: null, judge: null } },
  evaluation: { cases: [], runs: [], selectedCaseId: "", lastRun: null, loading: false, error: "" }
};

api.setTokens(state.accessToken, state.refreshToken);

void boot();

async function boot(): Promise<void> {
  if (state.accessToken) {
    try {
      state.user = await api.me();
      await loadInitialData();
    } catch {
      clearAuth();
    }
  }
  render();
}

async function loadInitialData(): Promise<void> {
  await loadSkills();
  if (state.view === "dashboard") await loadDashboard();
  if (state.view === "coding") await loadCoding();
  if (state.view === "memory") await loadMemory();
  if (state.view === "admin") await loadAdmin();
  if (state.view === "evaluation") await loadEvaluation();
}

function render(): void {
  appRoot.innerHTML = localize(state.user ? renderShell() : renderLogin());
  mountIcons();
  bindEvents();
}

function mountIcons(): void {
  createIcons({
    icons: iconSet,
    attrs: {
      width: "18",
      height: "18",
      "stroke-width": "2.2"
    } as Record<string, string>
  });
}

function icon(name: string, className = "ui-icon"): string {
  return `<i class="${className}" data-lucide="${name}" aria-hidden="true"></i>`;
}

function renderLogin(): string {
  return `
    <main class="login-screen">
      <section class="login-panel">
        <div class="brand-row">
          <span class="brand-mark">${icon("brain-circuit", "brand-icon")}</span>
          <div>
            <h1>InterviewOS</h1>
            <p>AI training runtime control surface</p>
          </div>
        </div>
        ${languageSwitcher("login-language")}
        <form id="login-form" class="login-form">
          <label>
            Email
            <input name="email" type="email" autocomplete="email" value="root@example.local" required />
          </label>
          <label>
            Password
            <input name="password" type="password" autocomplete="current-password" value="RootChangeMe123!" required />
          </label>
          <button class="button primary" type="submit">${icon("log-in")}<span>Sign in</span></button>
        </form>
        <p class="muted">Use the local root account from the backend bootstrap, or any user created through the API.</p>
      </section>
    </main>
  `;
}

function renderShell(): string {
  const busy = isCurrentViewLoading();
  return `
    <div class="app-shell">
      <aside class="sidebar">
        <div class="brand-row sidebar-brand">
          <span class="brand-mark">${icon("brain-circuit", "brand-icon")}</span>
          <div>
            <strong>InterviewOS</strong>
            <span>AI training runtime</span>
          </div>
        </div>
        <nav class="nav-list">
          ${navItem("dashboard", "Dashboard", "Overview", "layout-dashboard")}
          ${navItem("interview", "Interview", "Session", "messages-square")}
          ${navItem("coding", "Coding", "Practice", "code-2")}
          ${navItem("memory", "Memory", "Review", "database")}
          ${navItem("admin", "Admin", "Console", "settings-2")}
          ${navItem("evaluation", "Evaluation", "Quality", "clipboard-check")}
        </nav>
        <div class="sidebar-health">
          <strong>Runtime health</strong>
          <div class="chip-row">
            <span class="chip ok">Go API</span>
            <span class="chip info">Worker</span>
          </div>
          <small>Poll with the dashboard refresh action.</small>
        </div>
      </aside>
      <section class="workspace">
        <header class="topbar">
          <div>
            <h1>${pageTitle(state.view)}</h1>
            <p>${pageSubtitle(state.view)}</p>
          </div>
          <div class="topbar-actions">
            <button class="button ghost" data-action="refresh" ${busy ? "disabled" : ""}>${icon("refresh-cw")}<span>${busy ? "Updating" : "Refresh"}</span></button>
            ${languageSwitcher("topbar-language")}
            <div class="user-pill">${escapeHtml(state.user?.display_name ?? "User")} · ${escapeHtml(state.user?.role ?? "user")}</div>
            <button class="icon-button" data-action="logout" aria-label="Sign out">${icon("log-out")}</button>
          </div>
        </header>
        ${state.toast ? `<div class="toast">${escapeHtml(state.toast)}</div>` : ""}
        ${renderInteractionStrip()}
        <main class="page-body" aria-busy="${busy ? "true" : "false"}">${renderPage()}</main>
      </section>
    </div>
  `;
}

function navItem(view: View, label: string, sub: string, iconName: string): string {
  const active = state.view === view ? " active" : "";
  return `
    <button class="nav-item${active}" data-view="${view}">
      <span class="nav-mark">${icon(iconName)}</span>
      <span><strong>${label}</strong><small>${sub}</small></span>
    </button>
  `;
}

function renderPage(): string {
  switch (state.view) {
    case "dashboard":
      return renderDashboard();
    case "interview":
      return renderInterview();
    case "coding":
      return renderCoding();
    case "memory":
      return renderMemory();
    case "admin":
      return renderAdmin();
    case "evaluation":
      return renderEvaluation();
  }
}

function renderDashboard(): string {
  const data = state.dashboard.data;
  const worker = data.worker;
  const queue = record(worker?.queue);
  const outbox = record(worker?.outbox);
  const judge = data.judge;
  return `
    ${state.dashboard.error ? banner(state.dashboard.error, "error") : ""}
    <section class="console-hero">
      <div class="console-copy">
        <span class="eyebrow">Control plane</span>
        <h2>Keep the training loop visible</h2>
        <p>Start from the work that changes state: create an interview, submit code, review memory, then use evaluation runs to catch regressions before the loop drifts.</p>
        <div class="hero-actions">
          <button class="button primary" data-view="interview">${icon("messages-square")}<span>Start interview</span></button>
          <button class="button secondary" data-view="coding">${icon("code-2")}<span>Practice coding</span></button>
          <button class="button secondary" data-view="memory">${icon("database")}<span>Review memory</span></button>
        </div>
      </div>
      <div class="console-panel">
        ${operationRow("API boundary", stringValue(data.health?.status, "unknown"), "healthz")}
        ${operationRow("Async worker", stringValue(queue.pending, "0"), "pending stream items")}
        ${operationRow("Judge lane", stringValue(record(judge).queued, "0"), "queued submissions")}
      </div>
    </section>
    <section class="metric-grid">
      ${metricCard("API status", stringValue(data.health?.status, "unknown"), "healthz", "teal", "terminal")}
      ${metricCard("Queue pending", stringValue(queue.pending, "0"), "Redis Stream", "blue", "messages-square")}
      ${metricCard("Outbox pending", stringValue(outbox.pending, "0"), "PostgreSQL outbox", "amber", "database")}
      ${metricCard("Judge queued", stringValue(record(judge).queued, "0"), "coding submissions", "cyan", "code-2")}
    </section>
    <section class="content-grid two">
      <article class="card">
        <div class="section-head">
          <div><h2>Recent coding submissions</h2><p>Latest judge-facing records for the signed-in user.</p></div>
        </div>
        ${table(
          ["Question", "Language", "Status", "Score"],
          data.submissions.map((item) => [item.question_id, item.language, statusBadge(item.status), String(item.score)])
        )}
      </article>
      <article class="card">
        <div class="section-head">
          <div><h2>Evaluation runs</h2><p>Quality gates from the backend evaluation harness.</p></div>
        </div>
        ${table(
          ["Case", "Task", "Status", "Score"],
          data.evalRuns.map((item) => [item.case_id, item.task_type, statusBadge(item.status), String(Math.round(item.score))])
        )}
      </article>
    </section>
    <section class="card">
      <div class="section-head">
        <div><h2>Runtime timeline</h2><p>The front end should make asynchronous state visible instead of hiding backend work.</p></div>
      </div>
      <div class="timeline">
        ${["Answer queued", "Worker claimed", "Runtime scored", "Report generated", "Memory review"].map((label, index) => `
          <div class="timeline-step ${index < 3 ? "done" : ""}"><span></span><strong>${label}</strong></div>
        `).join("")}
      </div>
    </section>
  `;
}

function renderInterview(): string {
  const session = state.interview.session;
  return `
    ${state.interview.error ? banner(state.interview.error, "error") : ""}
    <section class="live-room">
      <div class="live-room-head">
        <div>
          <span class="eyebrow">Live practice room</span>
          <h2>Run the interview like a focused call</h2>
          <p>The layout follows meeting products: a main stage, participant tiles, a stable control bar and a right rail for session state.</p>
        </div>
        <div class="room-clock">
          ${icon("clock-3")}
          <span>${escapeHtml(formatTime(session?.updated_at))}</span>
        </div>
      </div>
      <div class="meeting-layout">
        <div class="meeting-main">
          ${renderLiveStage(session)}
          ${session ? renderSessionPanel(session) : emptyState("No active session", "Create a session to open the live interview room.")}
        </div>
        <aside class="meeting-sidebar">
          ${renderRoomCompanion(session)}
          ${renderSessionSetup(session)}
          ${renderEvaluationPanel(session)}
        </aside>
      </div>
    </section>
  `;
}

function renderLiveStage(session: InterviewSession | null): string {
  const question = session?.current_question;
  return `
    <div class="live-stage">
      ${renderStageStatusBar(session)}
      <div class="speaker-tile interviewer-tile">
        <div class="tile-bar">
          <span>${icon("bot")}<strong>AI Interviewer</strong></span>
          <b class="live-dot">${session ? "Live" : "Standby"}</b>
        </div>
        <div class="interviewer-avatar">${icon("brain-circuit", "avatar-icon")}</div>
        <div class="question-card">
          <div class="question-topline">
            <span class="eyebrow">Question ${session?.current_question_number || question?.number || 1}</span>
            ${statusBadge(session?.flow_status ?? "waiting")}
          </div>
          <h3>${escapeHtml(question?.title ?? "Backend interview question")}</h3>
          <p>${escapeHtml(question?.prompt ?? "Create a session to receive the first backend-generated question.")}</p>
          <div class="question-meta">
            <span>${icon("flag")}<strong>${escapeHtml(session?.phase ?? "Waiting")}</strong></span>
            <span>${icon("messages-square")}<strong>${escapeHtml(`${session?.turns?.length ?? 0} turns`)}</strong></span>
            <span>${icon("refresh-cw")}<strong>${escapeHtml(`${session?.follow_up_count ?? 0}/${session?.max_follow_ups ?? 0} follow-ups`)}</strong></span>
          </div>
          <div class="chip-row">${(question?.tags ?? ["runtime", "trace", "mock"]).map((tag) => `<span class="chip">${escapeHtml(tag)}</span>`).join("")}</div>
        </div>
      </div>
      <div class="participant-grid">
        ${participantTile("Candidate", state.user?.display_name ?? "You", "user-round", state.meeting.cameraOn ? "Camera on" : "Camera off", state.meeting.micOn ? "Mic on" : "Muted")}
        ${participantTile("Runtime", session ? session.skill_id : "Waiting", "radio", session ? session.flow_status : "No session", "AI channel")}
      </div>
      ${renderLiveCaption(session)}
      <div class="meeting-control-bar" role="toolbar" aria-label="Meeting controls">
        ${meetingToggle("mic", state.meeting.micOn, state.meeting.micOn ? "mic" : "mic-off", state.meeting.micOn ? "Mic on" : "Muted")}
        ${meetingToggle("camera", state.meeting.cameraOn, state.meeting.cameraOn ? "video" : "video-off", state.meeting.cameraOn ? "Camera on" : "Camera off")}
        ${meetingToggle("captions", state.meeting.captionsOn, "captions", state.meeting.captionsOn ? "Captions on" : "Captions off")}
        ${meetingToggle("prompt", state.meeting.promptShared, "monitor-up", state.meeting.promptShared ? "Prompt shared" : "Share prompt")}
        <button class="call-end" data-action="finalize-session" ${session && !state.interview.loading ? "" : "disabled"}>${icon("phone-off")}<span>End</span></button>
      </div>
    </div>
  `;
}

function renderStageStatusBar(session: InterviewSession | null): string {
  return `
    <div class="stage-status-bar">
      ${stageSignal("Room", session ? "Connected" : "Standby", "radio", session ? "ok" : "idle")}
      ${stageSignal("Flow", session?.flow_status ?? "Waiting", "flag", session ? "info" : "idle")}
      ${stageSignal("Mode", state.interview.dryRun ? "Dry run" : "Real runtime", "settings-2", state.interview.dryRun ? "warn" : "ok")}
      ${stageSignal("Trace", `${state.interview.trace.length} records`, "file-search", state.interview.trace.length ? "info" : "idle")}
    </div>
  `;
}

function stageSignal(label: string, value: string, iconName: string, tone: "ok" | "info" | "warn" | "idle"): string {
  return `
    <div class="stage-signal ${tone}">
      <span>${icon(iconName)}</span>
      <div>
        <small>${escapeHtml(label)}</small>
        <strong>${escapeHtml(value)}</strong>
      </div>
    </div>
  `;
}

function renderLiveCaption(session: InterviewSession | null): string {
  const caption = state.meeting.captionsOn
    ? session?.current_question?.prompt ?? "Create a session to start the interviewer channel."
    : "Captions are hidden. Turn them on from the control bar.";
  return `
    <div class="live-caption ${state.meeting.captionsOn ? "" : "is-muted"}">
      <span>${icon("captions")}</span>
      <div>
        <small>${state.meeting.captionsOn ? "Live captions" : "Captions hidden"}</small>
        <p>${escapeHtml(caption)}</p>
      </div>
    </div>
  `;
}

function meetingToggle(control: string, active: boolean, iconName: string, label: string): string {
  return `
    <button
      class="call-control ${active ? "active" : "muted"}"
      type="button"
      data-action="toggle-meeting-control"
      data-control="${control}"
      aria-pressed="${active ? "true" : "false"}"
    >${icon(iconName)}<b>${label}</b></button>
  `;
}

function participantTile(role: string, name: string, iconName: string, stateText: string, caption: string): string {
  return `
    <div class="participant-tile">
      <div class="participant-avatar">${icon(iconName)}</div>
      <div>
        <small>${escapeHtml(role)}</small>
        <strong>${escapeHtml(name)}</strong>
        <span>${escapeHtml(caption)} · ${escapeHtml(stateText)}</span>
      </div>
    </div>
  `;
}

function renderRoomCompanion(session: InterviewSession | null): string {
  return `
    <article class="room-card companion-card">
      <div class="companion-tabs" role="tablist" aria-label="Room companion">
        ${roomTab("briefing", "Briefing", "panel-right-open")}
        ${roomTab("participants", "People", "users")}
        ${roomTab("notes", "Notes", "captions")}
      </div>
      ${state.meeting.panel === "briefing" ? renderBriefingPanel(session) : ""}
      ${state.meeting.panel === "participants" ? renderParticipantsPanel(session) : ""}
      ${state.meeting.panel === "notes" ? renderNotesPanel(session) : ""}
    </article>
  `;
}

function roomTab(panel: RoomPanel, label: string, iconName: string): string {
  const active = state.meeting.panel === panel;
  return `
    <button
      class="companion-tab ${active ? "active" : ""}"
      type="button"
      data-action="set-room-panel"
      data-panel="${panel}"
      aria-selected="${active ? "true" : "false"}"
    >${icon(iconName)}<span>${label}</span></button>
  `;
}

function renderBriefingPanel(session: InterviewSession | null): string {
  return `
    <div class="companion-panel">
      <div class="briefing-card primary">
        <small>Current focus</small>
        <strong>${escapeHtml(session?.current_question?.title ?? "Prepare the first question")}</strong>
        <p>${escapeHtml(session ? "Answer clearly, then poll for the runtime evaluation." : "Create a session to start the live practice room.")}</p>
      </div>
      <div class="briefing-grid">
        ${briefingItem("Mic", state.meeting.micOn ? "On" : "Muted", state.meeting.micOn ? "mic" : "mic-off")}
        ${briefingItem("Camera", state.meeting.cameraOn ? "On" : "Off", state.meeting.cameraOn ? "video" : "video-off")}
        ${briefingItem("Captions", state.meeting.captionsOn ? "On" : "Off", "captions")}
        ${briefingItem("Prompt", state.meeting.promptShared ? "Shared" : "Private", "monitor-up")}
      </div>
    </div>
  `;
}

function briefingItem(label: string, value: string, iconName: string): string {
  return `
    <div class="briefing-item">
      <span>${icon(iconName)}</span>
      <small>${escapeHtml(label)}</small>
      <strong>${escapeHtml(value)}</strong>
    </div>
  `;
}

function renderParticipantsPanel(session: InterviewSession | null): string {
  return `
    <div class="companion-panel participant-list">
      ${participantLine("AI Interviewer", "Host", "bot", session ? "Ready" : "Waiting")}
      ${participantLine(state.user?.display_name ?? "Candidate", "You", "user-round", state.meeting.micOn ? "Speaking ready" : "Muted")}
      ${participantLine(session?.skill_id ?? "Runtime", "Evaluator", "radio", session?.flow_status ?? "No session")}
    </div>
  `;
}

function participantLine(name: string, role: string, iconName: string, status: string): string {
  return `
    <div class="participant-line">
      <span>${icon(iconName)}</span>
      <div>
        <strong>${escapeHtml(name)}</strong>
        <small>${escapeHtml(role)} · ${escapeHtml(status)}</small>
      </div>
    </div>
  `;
}

function renderNotesPanel(session: InterviewSession | null): string {
  return `
    <div class="companion-panel">
      <div class="caption-line">
        <span>${icon("captions")}</span>
        <p>${escapeHtml(state.meeting.captionsOn ? "Captions are ready for ASR/TTS integration." : "Captions are hidden. Turn them on from the control bar.")}</p>
      </div>
      <textarea class="room-notes" data-role="meeting-notes" rows="6" placeholder="Write notes for follow-up, evidence, or weak points.">${escapeHtml(state.meeting.notes)}</textarea>
      <small class="note-hint">${escapeHtml(session ? `Session ${session.session_id}` : "Notes are local until a backend notes API is added.")}</small>
    </div>
  `;
}

function renderSessionSetup(session: InterviewSession | null): string {
  return `
    <article class="room-card setup-card">
      <div class="section-head compact">
        <div><h2>Session setup</h2><p>Choose the room context before the first question is generated.</p></div>
        <button class="button secondary" data-action="poll-session" ${session && !state.interview.loading ? "" : "disabled"}>${icon("rotate-cw")}<span>${state.interview.loading ? "Updating" : "Poll"}</span></button>
      </div>
      <form id="session-form" class="setup-form" aria-busy="${state.interview.loading ? "true" : "false"}">
        <label>Skill
          <select name="skill_id">${skillOptions(session?.skill_id)}</select>
        </label>
        <label>Question type
          <select name="question_type">
            <option value="backend">backend</option>
            <option value="algorithm">algorithm</option>
            <option value="system_design">system_design</option>
          </select>
        </label>
        <label>Follow-ups
          <input name="max_follow_ups" type="number" min="0" max="5" value="${session?.max_follow_ups ?? 1}" />
        </label>
        <button class="button primary" type="submit" ${state.interview.loading ? "disabled" : ""}>${icon("plus")}<span>${state.interview.loading ? "Creating" : "Create session"}</span></button>
      </form>
    </article>
  `;
}

function renderEvaluationPanel(session: InterviewSession | null): string {
  return `
    <article class="room-card state-card">
      <div class="section-head compact">
        <div><h2>Room state</h2><p>Track the backend-owned state machine while the call is in progress.</p></div>
      </div>
      ${session ? `
        <div class="state-stack">
          ${statePill("Flow", session.flow_status, "messages-square")}
          ${statePill("Status", session.session_status, "terminal")}
          ${statePill("Phase", session.phase, "clipboard-check")}
          ${statePill("Total score", String(session.total_score), "file-text")}
        </div>
        <dl class="detail-list compact">
          <div><dt>Session</dt><dd>${escapeHtml(session.session_id)}</dd></div>
          <div><dt>Updated</dt><dd>${escapeHtml(formatTime(session.updated_at))}</dd></div>
        </dl>
      ` : emptyState("Waiting for session", "Trace and report controls appear after creation.")}
      <div class="button-row">
        <button class="button secondary" data-action="load-trace" ${session && !state.interview.loading ? "" : "disabled"}>${icon("file-search")}<span>Load trace</span></button>
        <button class="button secondary" data-action="generate-report" ${session && !state.interview.loading ? "" : "disabled"}>${icon("file-text")}<span>Generate report</span></button>
      </div>
      ${state.interview.trace.length ? renderTracePreview(state.interview.trace) : ""}
      ${state.interview.report ? renderReportPreview(state.interview.report) : ""}
    </article>
  `;
}

function renderSessionPanel(session: InterviewSession): string {
  const answerLength = state.interview.answer.trim().length;
  return `
    <article class="answer-dock speaking-dock">
      <div class="speaker-composer-head">
        <div class="speaker-identity">
          <span>${icon("user-round")}</span>
          <div>
            <h2>Candidate response</h2>
            <p>Write the answer as if you are speaking in the live room. Submit when ready, then poll for the evaluation.</p>
          </div>
        </div>
        <div class="speaker-state ${state.meeting.micOn ? "active" : "muted"}">
          ${icon(state.meeting.micOn ? "mic" : "mic-off")}
          <div>
            <strong>${state.meeting.micOn ? "Speaking channel open" : "Muted locally"}</strong>
            <small>${state.meeting.cameraOn ? "Camera on" : "Camera off"}</small>
          </div>
        </div>
      </div>
      <form id="answer-form" class="answer-form" aria-busy="${state.interview.loading ? "true" : "false"}">
        <div class="answer-editor-shell">
          <label class="answer-label">Your answer
            <textarea name="answer" rows="8" placeholder="Answer with concrete tradeoffs, examples, and follow-up hooks.">${escapeHtml(state.interview.answer)}</textarea>
          </label>
          <div class="answer-cue-strip" aria-live="polite">
            <span>${icon("messages-square")}<strong data-role="answer-length">${answerLength} characters</strong></span>
            <span>${icon("flag")}<strong>Round ${session.current_question_number}.${session.answer_round}</strong></span>
            <span>${icon("settings-2")}<strong>${state.interview.dryRun ? "Dry run protected" : "Real runtime enabled"}</strong></span>
          </div>
        </div>
        <div class="answer-actions speaker-actions">
          <label class="runtime-toggle"><input name="dry_run" type="checkbox" ${state.interview.dryRun ? "checked" : ""} /><span>${icon("settings-2")} Dry run runtime calls</span></label>
          <button class="button primary submit-response" type="submit" ${state.interview.loading ? "disabled" : ""}>${icon("send")}<span>${state.interview.loading ? "Sending" : "Submit answer"}</span></button>
        </div>
      </form>
      ${session.turns?.length ? renderTurns(session.turns) : ""}
    </article>
  `;
}

function renderTurns(turns: InterviewSession["turns"]): string {
  if (!turns?.length) return "";
  return `
    <div class="turn-list rich">
      <h3>Turn history</h3>
      ${turns.map((turn) => `
        <div class="turn-row">
          <div>
            <strong>Turn ${turn.question_number}.${turn.answer_round}</strong>
            <small>${escapeHtml(formatTime(turn.updated_at))}</small>
          </div>
          <div class="turn-meta">
            ${statusBadge(turn.turn_status)}
            <span>score ${turn.score}</span>
          </div>
          ${turn.error_text ? `<p>${escapeHtml(turn.error_text)}</p>` : ""}
        </div>
      `).join("")}
    </div>
  `;
}

function renderCoding(): string {
  const selected = state.coding.selectedQuestion;
  return `
    ${state.coding.error ? banner(state.coding.error, "error") : ""}
    <section class="content-grid coding-layout">
      <article class="card question-browser">
        <div class="section-head">
          <div><h2>Question set</h2><p>Published coding questions from PostgreSQL seed data.</p></div>
          <button class="button secondary" data-action="load-coding" ${state.coding.loading ? "disabled" : ""}>${icon("refresh-cw")}<span>${state.coding.loading ? "Updating" : "Reload"}</span></button>
        </div>
        <div class="list-panel">
          ${state.coding.questions.map((question) => `
            <button class="list-item ${selected?.question_id === question.question_id ? "active" : ""}" data-question-id="${escapeAttr(question.question_id)}">
              <strong>${escapeHtml(question.title)}</strong>
              <span>${escapeHtml(question.difficulty)} · ${escapeHtml(question.question_type)}</span>
              <small>${(question.topic_tags ?? []).slice(0, 3).map(escapeHtml).join(" / ")}</small>
            </button>
          `).join("") || emptyState("No questions", "Run migrations and seed data before using coding practice.", { label: "Reload", action: "load-coding", iconName: "refresh-cw" })}
        </div>
      </article>
      <article class="card editor-card code-studio">
        <div class="section-head">
          <div><h2>${escapeHtml(selected?.title ?? "Select a question")}</h2><p>${escapeHtml(selected?.constraints_text ?? "Prompt and constraints appear here.")}</p></div>
        </div>
        <div class="prompt-panel">
          <span class="eyebrow">Problem brief</span>
          <p class="prompt-text">${escapeHtml(selected?.prompt ?? "")}</p>
          <div class="chip-row">${(selected?.topic_tags ?? ["backend", "practice"]).slice(0, 5).map((tag) => `<span class="chip">${escapeHtml(tag)}</span>`).join("")}</div>
        </div>
        <form id="coding-form" class="coding-form" aria-busy="${state.coding.loading ? "true" : "false"}">
          <label>Language
            <select name="language">
              ${["go", "java", "python", "javascript", "typescript", "cpp"].map((lang) => `<option value="${lang}" ${state.coding.language === lang ? "selected" : ""}>${lang}</option>`).join("")}
            </select>
          </label>
          <textarea name="source_code" spellcheck="false">${escapeHtml(state.coding.sourceCode)}</textarea>
          <button class="button primary" type="submit" ${selected && !state.coding.loading ? "" : "disabled"}>${icon("terminal")}<span>${state.coding.loading ? "Submitting" : "Submit to judge"}</span></button>
        </form>
      </article>
      <aside class="card results-card">
        <div class="section-head compact">
          <div><h2>Judge results</h2><p>Latest asynchronous verdicts for this question.</p></div>
        </div>
        <div class="submission-list">
          ${state.coding.submissions.map(renderSubmissionCard).join("") || emptyState("No records", "Nothing has been returned by the API yet.")}
        </div>
      </aside>
    </section>
  `;
}

function renderMemory(): string {
  const runtime = record(state.memory.data.runtime_response);
  const candidates = extractItems(runtime);
  return `
    ${state.memory.error ? banner(state.memory.error, "error") : ""}
    <section class="content-grid two-wide">
      <article class="card">
        <div class="section-head">
          <div><h2>Pending memory candidates</h2><p>Human review stays between runtime extraction and long-term profile admission.</p></div>
          <button class="button secondary" data-action="load-memory" ${state.memory.loading ? "disabled" : ""}>${icon("refresh-cw")}<span>${state.memory.loading ? "Updating" : "Reload"}</span></button>
        </div>
        ${candidates.length ? candidates.map(renderMemoryCandidate).join("") : emptyState("No runtime candidates", "Start Python Runtime and load pending candidates to review memory.", { label: "Reload", action: "load-memory", iconName: "refresh-cw" })}
      </article>
      <aside class="card">
        <h2>Review rule</h2>
        <p class="muted">Only approved memory can enter prompt context. Reject weak, private, or hallucinated candidates and keep Go as the audit boundary.</p>
        <pre class="json-box">${escapeHtml(JSON.stringify(state.memory.data, null, 2))}</pre>
      </aside>
    </section>
  `;
}

function renderMemoryCandidate(item: JsonObject): string {
  const id = stringValue(item.candidate_id ?? item.id, "");
  return `
    <div class="candidate-row">
      <div>
        <strong>${escapeHtml(stringValue(item.topic, "Untitled candidate"))}</strong>
        <p>${escapeHtml(stringValue(item.content, "No content returned by runtime."))}</p>
        <small>confidence ${escapeHtml(stringValue(item.confidence, "n/a"))}</small>
      </div>
      <div class="button-row">
        <button class="button secondary" data-action="approve-memory" data-candidate-id="${escapeAttr(id)}" ${id && !state.memory.loading ? "" : "disabled"}>${icon("check")}<span>Approve</span></button>
        <button class="button danger" data-action="reject-memory" data-candidate-id="${escapeAttr(id)}" ${id && !state.memory.loading ? "" : "disabled"}>${icon("x")}<span>Reject</span></button>
      </div>
    </div>
  `;
}

function renderAdmin(): string {
  const data = state.admin.data;
  return `
    ${state.admin.error ? banner(state.admin.error, "error") : ""}
    <section class="content-grid two">
      <article class="card">
        <div class="section-head">
          <div><h2>Provider routes</h2><p>Root-only provider and task routing state.</p></div>
          <button class="button secondary" data-action="load-admin" ${state.admin.loading ? "disabled" : ""}>${icon("refresh-cw")}<span>${state.admin.loading ? "Updating" : "Reload"}</span></button>
        </div>
        ${table(
          ["Task", "Provider", "Fallback"],
          data.routes.map((item) => [stringValue(item.task_type), stringValue(item.provider_id), stringValue(item.fallback_provider_id, "-")])
        )}
      </article>
      <article class="card">
        <h2>Providers</h2>
        ${table(
          ["Provider", "Type", "Enabled"],
          data.providers.map((item) => [stringValue(item.provider_id), stringValue(item.provider_type), statusBadge(String(Boolean(item.enabled)))])
        )}
      </article>
    </section>
    <section class="content-grid two">
      <article class="card"><h2>Worker summary</h2><pre class="json-box">${escapeHtml(JSON.stringify(data.worker, null, 2))}</pre></article>
      <article class="card"><h2>Coding judge</h2><pre class="json-box">${escapeHtml(JSON.stringify(data.judge, null, 2))}</pre></article>
    </section>
  `;
}

function renderEvaluation(): string {
  return `
    ${state.evaluation.error ? banner(state.evaluation.error, "error") : ""}
    <section class="content-grid two-wide">
      <article class="card">
        <div class="section-head">
          <div><h2>Evaluation cases</h2><p>Store quality samples and run them through the runtime task path.</p></div>
          <button class="button secondary" data-action="load-evaluation" ${state.evaluation.loading ? "disabled" : ""}>${icon("refresh-cw")}<span>${state.evaluation.loading ? "Updating" : "Reload"}</span></button>
        </div>
        <form id="evaluation-form" class="evaluation-form" aria-busy="${state.evaluation.loading ? "true" : "false"}">
          <input name="case_id" placeholder="case_id, optional" />
          <input name="suite" placeholder="suite" value="runtime-smoke" />
          <select name="task_type">
            <option value="question_generation">question_generation</option>
            <option value="answer_evaluation">answer_evaluation</option>
            <option value="summary">summary</option>
            <option value="memory_extraction">memory_extraction</option>
          </select>
          <select name="skill_id">${skillOptions("java-backend")}</select>
          <textarea name="user_input" rows="3">Generate one Redis recovery interview question.</textarea>
          <button class="button primary" type="submit" ${state.evaluation.loading ? "disabled" : ""}>${icon("save")}<span>${state.evaluation.loading ? "Saving" : "Save case"}</span></button>
        </form>
        ${state.evaluation.cases.map((item) => `
          <div class="case-row">
            <button class="list-item ${state.evaluation.selectedCaseId === item.case_id ? "active" : ""}" data-eval-case-id="${escapeAttr(item.case_id)}">
              <strong>${escapeHtml(item.case_id)}</strong>
              <span>${escapeHtml(item.suite)} · ${escapeHtml(item.task_type)}</span>
            </button>
            <button class="button secondary" data-action="run-evaluation" data-case-id="${escapeAttr(item.case_id)}" ${state.evaluation.loading ? "disabled" : ""}>${icon("play")}<span>Run dry</span></button>
          </div>
        `).join("") || emptyState("No cases", "Create a small smoke case first.")}
      </article>
      <aside class="card">
        <h2>Runs</h2>
        ${table(
          ["Case", "Status", "Score", "Duration"],
          state.evaluation.runs.map((item) => [item.case_id, statusBadge(item.status), String(Math.round(item.score)), `${item.duration_ms}ms`])
        )}
        ${state.evaluation.lastRun ? `<pre class="json-box">${escapeHtml(JSON.stringify(state.evaluation.lastRun, null, 2))}</pre>` : ""}
      </aside>
    </section>
  `;
}

function bindEvents(): void {
  document.querySelectorAll<HTMLButtonElement>("[data-view]").forEach((button) => {
    button.addEventListener("click", () => {
      const view = button.dataset.view as View;
      void navigate(view);
    });
  });

  document.querySelector<HTMLFormElement>("#login-form")?.addEventListener("submit", (event) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget as HTMLFormElement);
    void handleLogin(String(form.get("email") ?? ""), String(form.get("password") ?? ""));
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='set-locale']").forEach((button) => {
    button.addEventListener("click", () => {
      setLocale(normalizeLocale(button.dataset.locale));
    });
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='toggle-meeting-control']").forEach((button) => {
    button.addEventListener("click", () => {
      toggleMeetingControl(button.dataset.control ?? "");
    });
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='set-room-panel']").forEach((button) => {
    button.addEventListener("click", () => {
      setRoomPanel((button.dataset.panel as RoomPanel | undefined) ?? "briefing");
    });
  });
  document.querySelector<HTMLTextAreaElement>("[data-role='meeting-notes']")?.addEventListener("input", (event) => {
    state.meeting.notes = (event.currentTarget as HTMLTextAreaElement).value;
    localStorage.setItem("frontend:meeting:notes", state.meeting.notes);
  });

  document.querySelector<HTMLButtonElement>("[data-action='logout']")?.addEventListener("click", () => void handleLogout());
  document.querySelectorAll<HTMLButtonElement>("[data-action='refresh']").forEach((button) => {
    button.addEventListener("click", () => void refreshCurrentView());
  });
  document.querySelector<HTMLFormElement>("#session-form")?.addEventListener("submit", onSessionSubmit);
  document.querySelector<HTMLFormElement>("#answer-form")?.addEventListener("submit", onAnswerSubmit);
  document.querySelector<HTMLTextAreaElement>("textarea[name='answer']")?.addEventListener("input", (event) => {
    state.interview.answer = (event.currentTarget as HTMLTextAreaElement).value;
    const answerLength = document.querySelector<HTMLElement>("[data-role='answer-length']");
    if (answerLength) {
      answerLength.textContent = `${state.interview.answer.trim().length} ${ui("characters")}`;
    }
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='poll-session']").forEach((button) => {
    button.addEventListener("click", () => void pollSession());
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='load-trace']").forEach((button) => {
    button.addEventListener("click", () => void loadTrace());
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='generate-report']").forEach((button) => {
    button.addEventListener("click", () => void generateReport());
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='finalize-session']").forEach((button) => {
    button.addEventListener("click", () => void finalizeSession());
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='load-coding']").forEach((button) => {
    button.addEventListener("click", () => void loadCoding());
  });
  document.querySelector<HTMLFormElement>("#coding-form")?.addEventListener("submit", onCodingSubmit);
  document.querySelector<HTMLSelectElement>("select[name='language']")?.addEventListener("change", (event) => {
    state.coding.language = (event.currentTarget as HTMLSelectElement).value;
  });
  document.querySelector<HTMLTextAreaElement>("textarea[name='source_code']")?.addEventListener("input", (event) => {
    state.coding.sourceCode = (event.currentTarget as HTMLTextAreaElement).value;
  });
  document.querySelectorAll<HTMLButtonElement>("[data-question-id]").forEach((button) => {
    button.addEventListener("click", () => void selectQuestion(button.dataset.questionId ?? ""));
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='load-memory']").forEach((button) => {
    button.addEventListener("click", () => void loadMemory());
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='approve-memory']").forEach((button) => {
    button.addEventListener("click", () => void reviewMemory(button.dataset.candidateId ?? "", "approve"));
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='reject-memory']").forEach((button) => {
    button.addEventListener("click", () => void reviewMemory(button.dataset.candidateId ?? "", "reject"));
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='load-admin']").forEach((button) => {
    button.addEventListener("click", () => void loadAdmin());
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='load-evaluation']").forEach((button) => {
    button.addEventListener("click", () => void loadEvaluation());
  });
  document.querySelector<HTMLFormElement>("#evaluation-form")?.addEventListener("submit", onEvaluationSubmit);
  document.querySelectorAll<HTMLButtonElement>("[data-eval-case-id]").forEach((button) => {
    button.addEventListener("click", () => {
      state.evaluation.selectedCaseId = button.dataset.evalCaseId ?? "";
      render();
    });
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='run-evaluation']").forEach((button) => {
    button.addEventListener("click", () => void runEvaluation(button.dataset.caseId ?? ""));
  });
}

async function navigate(view: View): Promise<void> {
  state.view = view;
  localStorage.setItem("frontend:view", view);
  render();
  await refreshCurrentView();
}

async function refreshCurrentView(): Promise<void> {
  if (state.view === "dashboard") await loadDashboard();
  if (state.view === "coding") await loadCoding();
  if (state.view === "memory") await loadMemory();
  if (state.view === "admin") await loadAdmin();
  if (state.view === "evaluation") await loadEvaluation();
  if (state.view === "interview" && state.interview.session) await pollSession();
}

async function handleLogin(email: string, password: string): Promise<void> {
  if (!email.trim() || !password.trim()) {
    toast(ui("Enter email and password"));
    return;
  }
  try {
    const auth = await api.login(email, password);
    saveAuth(auth);
    await loadInitialData();
    toast(ui("Signed in"));
  } catch (error) {
    toast(errorMessage(error));
  }
  render();
}

async function handleLogout(): Promise<void> {
  try {
    await api.logout();
  } catch {
    // local logout still succeeds
  }
  clearAuth();
  render();
}

async function loadSkills(): Promise<void> {
  try {
    state.skills = await api.listSkills();
  } catch {
    state.skills = [];
  }
}

async function loadDashboard(): Promise<void> {
  state.dashboard.loading = true;
  state.dashboard.error = "";
  render();
  const [health, worker, judge, evalRuns, submissions] = await Promise.allSettled([
    api.health(),
    api.workerSummary(),
    api.codingJudgeSummary(),
    api.listEvaluationRuns(),
    api.listCodingSubmissions()
  ]);
  state.dashboard.data = {
    health: settledValue(health, null),
    worker: settledValue(worker, null),
    judge: settledValue(judge, null),
    evalRuns: settledValue(evalRuns, []),
    submissions: settledValue(submissions, [])
  };
  state.dashboard.error = firstError([health, worker, judge, evalRuns, submissions]);
  state.dashboard.loading = false;
  render();
}

async function loadCoding(): Promise<void> {
  state.coding.loading = true;
  state.coding.error = "";
  render();
  try {
    state.coding.questions = await api.listCodingQuestions();
    if (!state.coding.selectedQuestion && state.coding.questions[0]) {
      await selectQuestion(state.coding.questions[0].question_id, false);
    }
    state.coding.submissions = await api.listCodingSubmissions(state.coding.selectedQuestion?.question_id ?? "");
  } catch (error) {
    state.coding.error = errorMessage(error);
  }
  state.coding.loading = false;
  render();
}

async function selectQuestion(questionId: string, shouldRender = true): Promise<void> {
  const fallback = state.coding.questions.find((item) => item.question_id === questionId) ?? null;
  state.coding.selectedQuestion = fallback;
  try {
    state.coding.selectedQuestion = await api.getCodingQuestion(questionId);
    state.coding.submissions = await api.listCodingSubmissions(questionId);
  } catch (error) {
    state.coding.error = errorMessage(error);
  }
  if (shouldRender) render();
}

async function loadMemory(): Promise<void> {
  state.memory.loading = true;
  state.memory.error = "";
  render();
  try {
    state.memory.data = await api.listMemoryCandidates("pending");
  } catch (error) {
    state.memory.error = errorMessage(error);
    state.memory.data = {};
  }
  state.memory.loading = false;
  render();
}

async function loadAdmin(): Promise<void> {
  state.admin.loading = true;
  state.admin.error = "";
  render();
  const [providers, routes, worker, judge] = await Promise.allSettled([
    api.listProviders(),
    api.listProviderRoutes(),
    api.workerSummary(),
    api.codingJudgeSummary()
  ]);
  state.admin.data = {
    providers: settledValue(providers, []),
    routes: settledValue(routes, []),
    worker: settledValue(worker, null),
    judge: settledValue(judge, null)
  };
  state.admin.error = firstError([providers, routes, worker, judge]);
  state.admin.loading = false;
  render();
}

async function loadEvaluation(): Promise<void> {
  state.evaluation.loading = true;
  state.evaluation.error = "";
  render();
  const [cases, runs] = await Promise.allSettled([api.listEvaluationCases(), api.listEvaluationRuns()]);
  state.evaluation.cases = settledValue(cases, []);
  state.evaluation.runs = settledValue(runs, []);
  state.evaluation.error = firstError([cases, runs]);
  state.evaluation.loading = false;
  render();
}

async function onSessionSubmit(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  if (state.interview.loading) return;
  const form = new FormData(event.currentTarget as HTMLFormElement);
  const maxFollowUps = Number(form.get("max_follow_ups") ?? 1);
  if (!Number.isFinite(maxFollowUps) || maxFollowUps < 0 || maxFollowUps > 5) {
    toast(ui("Follow-ups must be between 0 and 5"));
    return;
  }
  state.interview.loading = true;
  state.interview.error = "";
  render();
  try {
    state.interview.session = await api.createInterviewSession(
      String(form.get("skill_id") ?? "java-backend"),
      String(form.get("question_type") ?? "backend"),
      maxFollowUps
    );
    toast(ui("Session created"));
  } catch (error) {
    state.interview.error = errorMessage(error);
  } finally {
    state.interview.loading = false;
  }
  render();
}

async function onAnswerSubmit(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  if (!state.interview.session) return;
  if (state.interview.loading) return;
  const form = new FormData(event.currentTarget as HTMLFormElement);
  state.interview.answer = String(form.get("answer") ?? "");
  state.interview.dryRun = form.get("dry_run") === "on";
  if (!state.interview.answer.trim()) {
    state.interview.error = "Write an answer before sending it.";
    toast(ui("Write an answer before sending it."));
    render();
    return;
  }
  state.interview.loading = true;
  state.interview.error = "";
  render();
  try {
    await api.submitInterviewAnswer(state.interview.session, state.interview.answer, state.interview.dryRun);
    toast(ui("Answer accepted by API"));
    state.interview.session = await api.getInterviewSession(state.interview.session.session_id);
  } catch (error) {
    state.interview.error = errorMessage(error);
  } finally {
    state.interview.loading = false;
  }
  render();
}

async function pollSession(): Promise<void> {
  if (!state.interview.session) return;
  if (state.interview.loading) return;
  state.interview.loading = true;
  state.interview.error = "";
  render();
  try {
    state.interview.session = await api.getInterviewSession(state.interview.session.session_id);
  } catch (error) {
    state.interview.error = errorMessage(error);
  } finally {
    state.interview.loading = false;
  }
  render();
}

async function loadTrace(): Promise<void> {
  if (!state.interview.session) return;
  if (state.interview.loading) return;
  state.interview.loading = true;
  state.interview.error = "";
  render();
  try {
    state.interview.trace = await api.getInterviewTrace(state.interview.session.session_id);
  } catch (error) {
    state.interview.error = errorMessage(error);
  } finally {
    state.interview.loading = false;
  }
  render();
}

async function generateReport(): Promise<void> {
  if (!state.interview.session) return;
  if (state.interview.loading) return;
  state.interview.loading = true;
  state.interview.error = "";
  render();
  try {
    state.interview.report = await api.generateInterviewReport(state.interview.session.session_id, true);
  } catch (error) {
    state.interview.error = errorMessage(error);
  } finally {
    state.interview.loading = false;
  }
  render();
}

async function finalizeSession(): Promise<void> {
  if (!state.interview.session) return;
  if (state.interview.loading) return;
  const confirmed = window.confirm(ui("Finalize this interview session?"));
  if (!confirmed) return;
  state.interview.loading = true;
  state.interview.error = "";
  render();
  try {
    state.interview.session = await api.finalizeInterviewSession(state.interview.session.session_id);
  } catch (error) {
    state.interview.error = errorMessage(error);
  } finally {
    state.interview.loading = false;
  }
  render();
}

async function onCodingSubmit(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  if (!state.coding.selectedQuestion) return;
  if (state.coding.loading) return;
  const form = new FormData(event.currentTarget as HTMLFormElement);
  state.coding.language = String(form.get("language") ?? "go");
  state.coding.sourceCode = String(form.get("source_code") ?? "");
  if (!state.coding.sourceCode.trim()) {
    state.coding.error = "Write code before sending it to the judge.";
    toast(ui("Write code before sending it to the judge."));
    render();
    return;
  }
  state.coding.loading = true;
  state.coding.error = "";
  render();
  try {
    await api.createCodingSubmission(state.coding.selectedQuestion.question_id, state.coding.language, state.coding.sourceCode);
    toast(ui("Submission queued"));
    state.coding.submissions = await api.listCodingSubmissions(state.coding.selectedQuestion.question_id);
  } catch (error) {
    state.coding.error = errorMessage(error);
  } finally {
    state.coding.loading = false;
  }
  render();
}

async function reviewMemory(candidateId: string, action: "approve" | "reject"): Promise<void> {
  if (!candidateId) return;
  if (state.memory.loading) return;
  state.memory.loading = true;
  state.memory.error = "";
  render();
  try {
    if (action === "approve") await api.approveMemoryCandidate(candidateId, "Approved from frontend review");
    else await api.rejectMemoryCandidate(candidateId, "Rejected from frontend review");
    toast(ui(action === "approve" ? "Memory approved" : "Memory rejected"));
    await loadMemory();
  } catch (error) {
    state.memory.error = errorMessage(error);
  } finally {
    state.memory.loading = false;
    render();
  }
}

async function onEvaluationSubmit(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  if (state.evaluation.loading) return;
  const form = new FormData(event.currentTarget as HTMLFormElement);
  const userInput = String(form.get("user_input") ?? "");
  if (!userInput.trim()) {
    state.evaluation.error = "Write an evaluation input before saving the case.";
    toast(ui("Write an evaluation input before saving the case."));
    render();
    return;
  }
  state.evaluation.loading = true;
  state.evaluation.error = "";
  render();
  try {
    await api.saveEvaluationCase({
      case_id: String(form.get("case_id") ?? ""),
      suite: String(form.get("suite") ?? "runtime-smoke"),
      task_type: String(form.get("task_type") ?? "question_generation"),
      skill_id: String(form.get("skill_id") ?? "java-backend"),
      input: { user_input: userInput },
      expected: {
        required_fields: ["question"],
        contains: { question: "Redis" }
      },
      tags: ["frontend", "smoke"],
      status: "active"
    });
    toast(ui("Evaluation case saved"));
    await loadEvaluation();
  } catch (error) {
    state.evaluation.error = errorMessage(error);
  } finally {
    state.evaluation.loading = false;
    render();
  }
}

async function runEvaluation(caseId: string): Promise<void> {
  if (!caseId) {
    toast(ui("Choose a case to run."));
    return;
  }
  if (state.evaluation.loading) return;
  state.evaluation.loading = true;
  state.evaluation.error = "";
  render();
  try {
    state.evaluation.lastRun = await api.runEvaluationCase(caseId, true);
    toast(ui("Evaluation run recorded"));
    await loadEvaluation();
  } catch (error) {
    state.evaluation.error = errorMessage(error);
  } finally {
    state.evaluation.loading = false;
    render();
  }
}

function saveAuth(auth: ApiState): void {
  state.user = auth.user;
  state.accessToken = auth.accessToken;
  state.refreshToken = auth.refreshToken;
  localStorage.setItem("frontend:access_token", auth.accessToken);
  localStorage.setItem("frontend:refresh_token", auth.refreshToken);
  api.setTokens(auth.accessToken, auth.refreshToken);
}

function clearAuth(): void {
  state.user = null;
  state.accessToken = "";
  state.refreshToken = "";
  localStorage.removeItem("frontend:access_token");
  localStorage.removeItem("frontend:refresh_token");
  api.clearTokens();
}

function toast(message: string): void {
  state.toast = message;
  window.setTimeout(() => {
    state.toast = "";
    render();
  }, 2600);
}

function setLocale(locale: Locale): void {
  state.locale = locale;
  localStorage.setItem("frontend:locale", locale);
  render();
}

function toggleMeetingControl(control: string): void {
  if (control === "mic") {
    state.meeting.micOn = !state.meeting.micOn;
    localStorage.setItem("frontend:meeting:mic", state.meeting.micOn ? "on" : "off");
    toast(ui(state.meeting.micOn ? "Microphone enabled" : "Microphone muted"));
  } else if (control === "camera") {
    state.meeting.cameraOn = !state.meeting.cameraOn;
    localStorage.setItem("frontend:meeting:camera", state.meeting.cameraOn ? "on" : "off");
    toast(ui(state.meeting.cameraOn ? "Camera enabled" : "Camera disabled"));
  } else if (control === "captions") {
    state.meeting.captionsOn = !state.meeting.captionsOn;
    localStorage.setItem("frontend:meeting:captions", state.meeting.captionsOn ? "on" : "off");
    toast(ui(state.meeting.captionsOn ? "Captions enabled" : "Captions hidden"));
  } else if (control === "prompt") {
    state.meeting.promptShared = !state.meeting.promptShared;
    localStorage.setItem("frontend:meeting:prompt_shared", state.meeting.promptShared ? "on" : "off");
    toast(ui(state.meeting.promptShared ? "Prompt shared" : "Prompt private"));
  }
  render();
}

function setRoomPanel(panel: RoomPanel): void {
  state.meeting.panel = normalizeRoomPanel(panel);
  localStorage.setItem("frontend:meeting:panel", state.meeting.panel);
  render();
}

function normalizeRoomPanel(panel: string | null | undefined): RoomPanel {
  if (panel === "briefing" || panel === "participants" || panel === "notes") {
    return panel;
  }
  return "briefing";
}

function localize(html: string): string {
  return translateHtml(state.locale, html);
}

function ui(value: string): string {
  return translate(state.locale, value);
}

function languageSwitcher(id: string): string {
  return `
    <div class="language-field" id="${id}" aria-label="Language preference">
      <span>Language preference</span>
      <div class="language-buttons" role="group" aria-label="Language preference">
        ${locales.map((locale) => `
          <button
            class="language-button ${locale.value === state.locale ? "active" : ""}"
            type="button"
            data-action="set-locale"
            data-locale="${locale.value}"
            aria-pressed="${locale.value === state.locale ? "true" : "false"}"
          >${locale.label}</button>
        `).join("")}
      </div>
    </div>
  `;
}

function skillOptions(selected = ""): string {
  const items = state.skills.length ? state.skills : [{ id: "java-backend", display_name: "Java backend" }];
  return items.map((skill) => {
    const id = skill.id ?? skill.skill_id ?? "java-backend";
    const label = skill.display_name ?? id;
    return `<option value="${escapeAttr(id)}" ${id === selected ? "selected" : ""}>${escapeHtml(label)}</option>`;
  }).join("");
}

function isCurrentViewLoading(): boolean {
  if (state.view === "dashboard") return state.dashboard.loading;
  if (state.view === "interview") return state.interview.loading;
  if (state.view === "coding") return state.coding.loading;
  if (state.view === "memory") return state.memory.loading;
  if (state.view === "admin") return state.admin.loading;
  return state.evaluation.loading;
}

function currentViewError(): string {
  if (state.view === "dashboard") return state.dashboard.error;
  if (state.view === "interview") return state.interview.error;
  if (state.view === "coding") return state.coding.error;
  if (state.view === "memory") return state.memory.error;
  if (state.view === "admin") return state.admin.error;
  return state.evaluation.error;
}

function renderInteractionStrip(): string {
  const busy = isCurrentViewLoading();
  const error = currentViewError();
  const tone = error ? "danger" : busy ? "info" : "ok";
  const title = error ? "Needs attention" : busy ? "Working" : "Ready";
  return `
    <section class="interaction-strip ${tone}" aria-live="polite">
      <div>
        <span>${icon(error ? "x" : busy ? "refresh-cw" : "check")}</span>
        <div>
          <strong>${title}</strong>
          <p>${escapeHtml(error || interactionHint())}</p>
        </div>
      </div>
      ${renderNextAction()}
    </section>
  `;
}

function interactionHint(): string {
  if (state.view === "dashboard") return "Choose the next training task or refresh the operational snapshot.";
  if (state.view === "interview") {
    if (!state.interview.session) return "Create a session, then answer the active question.";
    return "Answer, poll the session, then generate the report when the flow is complete.";
  }
  if (state.view === "coding") {
    if (!state.coding.selectedQuestion) return "Choose a question before sending code to the judge.";
    return "Edit code, submit it, then watch the asynchronous verdict.";
  }
  if (state.view === "memory") return "Review candidates one by one so only trusted memory reaches prompts.";
  if (state.view === "admin") return "Check provider routing and worker state before debugging runtime failures.";
  return "Save a sample case, run it, then inspect assertions and score changes.";
}

function renderNextAction(): string {
  if (isCurrentViewLoading()) return `<span class="mini-status">${icon("refresh-cw")}<b>Syncing</b></span>`;
  if (state.view === "dashboard") return `<button class="button secondary" data-view="interview">${icon("messages-square")}<span>Start interview</span></button>`;
  if (state.view === "interview" && state.interview.session) {
    return `<button class="button secondary" data-action="poll-session">${icon("rotate-cw")}<span>Poll session</span></button>`;
  }
  if (state.view === "coding") return `<button class="button secondary" data-action="load-coding">${icon("refresh-cw")}<span>Reload</span></button>`;
  if (state.view === "memory") return `<button class="button secondary" data-action="load-memory">${icon("refresh-cw")}<span>Reload</span></button>`;
  if (state.view === "evaluation") return `<button class="button secondary" data-action="load-evaluation">${icon("refresh-cw")}<span>Reload</span></button>`;
  return `<button class="button secondary" data-action="refresh">${icon("refresh-cw")}<span>Refresh</span></button>`;
}

function operationRow(label: string, value: string, hint: string): string {
  return `
    <div class="operation-row">
      <span>${icon("check")}</span>
      <div>
        <strong>${escapeHtml(label)}</strong>
        <small>${escapeHtml(hint)}</small>
      </div>
      <b>${escapeHtml(value)}</b>
    </div>
  `;
}

function statePill(label: string, value: string, iconName: string): string {
  return `
    <div class="state-pill">
      <span>${icon(iconName)}</span>
      <div>
        <small>${escapeHtml(label)}</small>
        <strong>${escapeHtml(value || "unknown")}</strong>
      </div>
    </div>
  `;
}

function renderTracePreview(trace: JsonObject[]): string {
  return `
    <div class="preview-block">
      <div class="preview-head"><strong>Trace preview</strong><small>${trace.length} records</small></div>
      ${trace.slice(0, 4).map((item) => `
        <div class="preview-row">
          <span>${icon("file-search")}</span>
          <div>
            <strong>${escapeHtml(stringValue(item.event_type ?? item.phase ?? item.trace_type, "runtime trace"))}</strong>
            <small>${escapeHtml(stringValue(item.created_at ?? item.updated_at ?? item.trace_id, "trace evidence"))}</small>
          </div>
        </div>
      `).join("")}
    </div>
  `;
}

function renderReportPreview(report: JsonObject): string {
  const content = record(report.content ?? report.output ?? report.runtime_response ?? report);
  const summary = stringValue(content.summary ?? content.overall_summary ?? content.feedback, "Report generated. Open the raw payload below if you need exact fields.");
  return `
    <div class="report-preview">
      <div class="preview-head"><strong>Report preview</strong>${statusBadge(stringValue(report.status, "report"))}</div>
      <p>${escapeHtml(summary)}</p>
      <pre class="json-box compact">${escapeHtml(JSON.stringify(report, null, 2))}</pre>
    </div>
  `;
}

function renderSubmissionCard(item: CodingSubmission): string {
  const result = record(item.result);
  const message = stringValue(result.message ?? result.error ?? result.verdict, "Awaiting judge details");
  return `
    <div class="submission-card">
      <div>
        <strong>${escapeHtml(item.language)}</strong>
        <small>${escapeHtml(formatTime(item.updated_at || item.created_at))}</small>
      </div>
      <div class="submission-score">
        ${statusBadge(item.status)}
        <b>${item.score}</b>
      </div>
      <p>${escapeHtml(message)}</p>
    </div>
  `;
}

function metricCard(label: string, value: string, hint: string, tone: string, iconName: string): string {
  return `
    <article class="metric-card ${tone}">
      <div class="metric-icon">${icon(iconName)}</div>
      <div>
        <span>${escapeHtml(label)}</span>
        <strong>${escapeHtml(value)}</strong>
        <small>${escapeHtml(hint)}</small>
      </div>
    </article>
  `;
}

function table(headers: string[], rows: string[][]): string {
  if (!rows.length) return emptyState("No records", "Nothing has been returned by the API yet.");
  return `
    <div class="table-wrap">
      <table>
        <thead><tr>${headers.map((header) => `<th>${escapeHtml(header)}</th>`).join("")}</tr></thead>
        <tbody>${rows.map((row) => `<tr>${row.map((cell) => `<td>${cell}</td>`).join("")}</tr>`).join("")}</tbody>
      </table>
    </div>
  `;
}

function statusBadge(status: string): string {
  const normalized = status.toLowerCase();
  const tone = ["passed", "completed", "accepted", "ready", "true", "ok", "enabled", "report"].includes(normalized)
    ? "ok"
    : ["failed", "error", "wrong_answer", "false", "disabled"].includes(normalized)
      ? "danger"
      : "info";
  return `<span class="status ${tone}">${escapeHtml(status)}</span>`;
}

function banner(message: string, type: "error" | "info"): string {
  return `<div class="banner ${type}">${escapeHtml(message)}</div>`;
}

function emptyState(title: string, copy: string, action?: EmptyAction): string {
  const button = action
    ? `<button class="button secondary" ${action.view ? `data-view="${action.view}"` : ""} ${action.action ? `data-action="${escapeAttr(action.action)}"` : ""}>${icon(action.iconName)}<span>${escapeHtml(action.label)}</span></button>`
    : "";
  return `<div class="empty-state"><strong>${escapeHtml(title)}</strong><p>${escapeHtml(copy)}</p>${button}</div>`;
}

function extractItems(value: JsonObject): JsonObject[] {
  const candidates = value.candidates ?? value.items ?? value.data;
  return Array.isArray(candidates) ? candidates.filter(isRecord) : [];
}

function record(value: unknown): JsonObject {
  return isRecord(value) ? value : {};
}

function stringValue(value: unknown, fallback = ""): string {
  if (value === null || value === undefined || value === "") return fallback;
  return String(value);
}

function formatTime(value: string | undefined): string {
  if (!value) return "not recorded";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(state.locale === "zh-CN" ? "zh-CN" : "en-US", {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  });
}

function settledValue<T>(result: PromiseSettledResult<T>, fallback: T): T {
  return result.status === "fulfilled" ? result.value : fallback;
}

function firstError(results: PromiseSettledResult<unknown>[]): string {
  const rejected = results.find((result) => result.status === "rejected") as PromiseRejectedResult | undefined;
  return rejected ? errorMessage(rejected.reason) : "";
}

function errorMessage(error: unknown): string {
  if (error instanceof ApiError) return `${error.code}: ${error.message}`;
  if (error instanceof Error) return error.message;
  return "Unknown error";
}

function pageTitle(view: View): string {
  return {
    dashboard: "Training dashboard",
    interview: "Interview session",
    coding: "Coding practice",
    memory: "Memory review",
    admin: "Admin console",
    evaluation: "Evaluation harness"
  }[view];
}

function pageSubtitle(view: View): string {
  return {
    dashboard: "Operational snapshot of sessions, queues, runtime health and quality gates.",
    interview: "Focused answer workflow with async evaluation, follow-up and trace evidence.",
    coding: "Question list, editor, sample tests and asynchronous judge results.",
    memory: "Human-in-the-loop approval for durable candidate memory.",
    admin: "Provider routing, workers and system operations.",
    evaluation: "Quality samples, dry-run checks and regression records."
  }[view];
}

function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function escapeAttr(value: string): string {
  return escapeHtml(value).replaceAll("`", "&#096;");
}

function isRecord(value: unknown): value is JsonObject {
  return typeof value === "object" && value !== null;
}

function defaultGoSolution(): string {
  return `package main

import "fmt"

func main() {
  fmt.Println("ready")
}
`;
}
