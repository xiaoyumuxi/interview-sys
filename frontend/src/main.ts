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

type View = "dashboard" | "interview" | "coding" | "memory" | "admin" | "evaluation";
type LoadState<T> = { loading: boolean; error: string; data: T };

interface AppState {
  view: View;
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
const root = document.querySelector<HTMLDivElement>("#app");

if (!root) {
  throw new Error("missing #app");
}
const appRoot = root;

const state: AppState = {
  view: (localStorage.getItem("frontend:view") as View | null) ?? "dashboard",
  user: null,
  accessToken: localStorage.getItem("frontend:access_token") ?? "",
  refreshToken: localStorage.getItem("frontend:refresh_token") ?? "",
  toast: "",
  skills: [],
  dashboard: { loading: false, error: "", data: { health: null, worker: null, judge: null, evalRuns: [], submissions: [] } },
  interview: { session: null, trace: [], report: null, answer: "", dryRun: true, error: "", loading: false },
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
  appRoot.innerHTML = state.user ? renderShell() : renderLogin();
  bindEvents();
}

function renderLogin(): string {
  return `
    <main class="login-screen">
      <section class="login-panel">
        <div class="brand-row">
          <span class="brand-mark"></span>
          <div>
            <h1>InterviewOS</h1>
            <p>AI training runtime control surface</p>
          </div>
        </div>
        <form id="login-form" class="login-form">
          <label>
            Email
            <input name="email" type="email" autocomplete="email" value="root@example.local" />
          </label>
          <label>
            Password
            <input name="password" type="password" autocomplete="current-password" value="RootChangeMe123!" />
          </label>
          <button class="button primary" type="submit">Sign in</button>
        </form>
        <p class="muted">Use the local root account from the backend bootstrap, or any user created through the API.</p>
      </section>
    </main>
  `;
}

function renderShell(): string {
  return `
    <div class="app-shell">
      <aside class="sidebar">
        <div class="brand-row sidebar-brand">
          <span class="brand-mark"></span>
          <div>
            <strong>InterviewOS</strong>
            <span>AI training runtime</span>
          </div>
        </div>
        <nav class="nav-list">
          ${navItem("dashboard", "Dashboard", "Overview", "D")}
          ${navItem("interview", "Interview", "Session", "I")}
          ${navItem("coding", "Coding", "Practice", "C")}
          ${navItem("memory", "Memory", "Review", "M")}
          ${navItem("admin", "Admin", "Console", "A")}
          ${navItem("evaluation", "Evaluation", "Quality", "E")}
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
            <button class="button ghost" data-action="refresh">Refresh</button>
            <div class="user-pill">${escapeHtml(state.user?.display_name ?? "User")} · ${escapeHtml(state.user?.role ?? "user")}</div>
            <button class="icon-button" data-action="logout" aria-label="Sign out">×</button>
          </div>
        </header>
        ${state.toast ? `<div class="toast">${escapeHtml(state.toast)}</div>` : ""}
        <main class="page-body">${renderPage()}</main>
      </section>
    </div>
  `;
}

function navItem(view: View, label: string, sub: string, mark: string): string {
  const active = state.view === view ? " active" : "";
  return `
    <button class="nav-item${active}" data-view="${view}">
      <span class="nav-mark">${mark}</span>
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
    <section class="metric-grid">
      ${metricCard("API status", stringValue(data.health?.status, "unknown"), "healthz", "teal")}
      ${metricCard("Queue pending", stringValue(queue.pending, "0"), "Redis Stream", "blue")}
      ${metricCard("Outbox pending", stringValue(outbox.pending, "0"), "PostgreSQL outbox", "amber")}
      ${metricCard("Judge queued", stringValue(record(judge).queued, "0"), "coding submissions", "cyan")}
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
  const current = session?.current_question;
  return `
    ${state.interview.error ? banner(state.interview.error, "error") : ""}
    <section class="content-grid two-wide">
      <article class="card">
        <div class="section-head">
          <div><h2>Start or resume session</h2><p>Create an interview session, answer the current question, then poll trace state.</p></div>
          <button class="button secondary" data-action="poll-session" ${session ? "" : "disabled"}>Poll session</button>
        </div>
        <form id="session-form" class="form-row">
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
            <input name="max_follow_ups" type="number" min="0" max="5" value="1" />
          </label>
          <button class="button primary" type="submit">Create session</button>
        </form>
        ${session ? renderSessionPanel(session) : emptyState("No active session", "Create a session to load the first backend-generated question.")}
      </article>
      <aside class="card">
        <h2>Evaluation state</h2>
        ${session ? `
          <dl class="detail-list">
            <div><dt>Session</dt><dd>${escapeHtml(session.session_id)}</dd></div>
            <div><dt>Flow</dt><dd>${statusBadge(session.flow_status)}</dd></div>
            <div><dt>Status</dt><dd>${statusBadge(session.session_status)}</dd></div>
            <div><dt>Total score</dt><dd>${session.total_score}</dd></div>
          </dl>
        ` : emptyState("Waiting for session", "Trace and report controls appear after creation.")}
        <div class="button-row">
          <button class="button secondary" data-action="load-trace" ${session ? "" : "disabled"}>Load trace</button>
          <button class="button secondary" data-action="generate-report" ${session ? "" : "disabled"}>Generate report</button>
          <button class="button danger" data-action="finalize-session" ${session ? "" : "disabled"}>Finalize</button>
        </div>
        ${state.interview.trace.length ? `<pre class="json-box">${escapeHtml(JSON.stringify(state.interview.trace.slice(0, 3), null, 2))}</pre>` : ""}
        ${state.interview.report ? `<pre class="json-box">${escapeHtml(JSON.stringify(state.interview.report, null, 2))}</pre>` : ""}
      </aside>
    </section>
  `;
}

function renderSessionPanel(session: InterviewSession): string {
  const question = session.current_question;
  return `
    <div class="question-card">
      <span class="eyebrow">Question ${session.current_question_number || question?.number || 1}</span>
      <h3>${escapeHtml(question?.title ?? "Backend interview question")}</h3>
      <p>${escapeHtml(question?.prompt ?? "The backend has not returned a current question yet. Poll the session after workers run.")}</p>
      <div class="chip-row">${(question?.tags ?? ["runtime", "trace"]).map((tag) => `<span class="chip">${escapeHtml(tag)}</span>`).join("")}</div>
    </div>
    <form id="answer-form" class="answer-form">
      <label>Your answer
        <textarea name="answer" rows="8">${escapeHtml(state.interview.answer)}</textarea>
      </label>
      <label class="check-row"><input name="dry_run" type="checkbox" ${state.interview.dryRun ? "checked" : ""} /> Dry run runtime calls</label>
      <button class="button primary" type="submit">Submit answer</button>
    </form>
    ${session.turns?.length ? renderTurns(session.turns) : ""}
  `;
}

function renderTurns(turns: InterviewSession["turns"]): string {
  if (!turns?.length) return "";
  return `
    <div class="turn-list">
      ${turns.map((turn) => `
        <div class="turn-row">
          <strong>Turn ${turn.question_number}.${turn.answer_round}</strong>
          ${statusBadge(turn.turn_status)}
          <span>score ${turn.score}</span>
          ${turn.error_text ? `<small>${escapeHtml(turn.error_text)}</small>` : ""}
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
      <article class="card">
        <div class="section-head">
          <div><h2>Question set</h2><p>Published coding questions from PostgreSQL seed data.</p></div>
          <button class="button secondary" data-action="load-coding">Reload</button>
        </div>
        <div class="list-panel">
          ${state.coding.questions.map((question) => `
            <button class="list-item ${selected?.question_id === question.question_id ? "active" : ""}" data-question-id="${escapeAttr(question.question_id)}">
              <strong>${escapeHtml(question.title)}</strong>
              <span>${escapeHtml(question.difficulty)} · ${escapeHtml(question.question_type)}</span>
            </button>
          `).join("") || emptyState("No questions", "Run migrations and seed data before using coding practice.")}
        </div>
      </article>
      <article class="card editor-card">
        <div class="section-head">
          <div><h2>${escapeHtml(selected?.title ?? "Select a question")}</h2><p>${escapeHtml(selected?.constraints_text ?? "Prompt and constraints appear here.")}</p></div>
        </div>
        <p class="prompt-text">${escapeHtml(selected?.prompt ?? "")}</p>
        <form id="coding-form" class="coding-form">
          <label>Language
            <select name="language">
              ${["go", "java", "python", "javascript", "typescript", "cpp"].map((lang) => `<option value="${lang}" ${state.coding.language === lang ? "selected" : ""}>${lang}</option>`).join("")}
            </select>
          </label>
          <textarea name="source_code" spellcheck="false">${escapeHtml(state.coding.sourceCode)}</textarea>
          <button class="button primary" type="submit" ${selected ? "" : "disabled"}>Submit to judge</button>
        </form>
      </article>
      <aside class="card">
        <h2>Judge results</h2>
        ${table(
          ["Status", "Score", "Language"],
          state.coding.submissions.map((item) => [statusBadge(item.status), String(item.score), item.language])
        )}
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
          <button class="button secondary" data-action="load-memory">Reload</button>
        </div>
        ${candidates.length ? candidates.map(renderMemoryCandidate).join("") : emptyState("No runtime candidates", "Start Python Runtime and load pending candidates to review memory.")}
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
        <button class="button secondary" data-action="approve-memory" data-candidate-id="${escapeAttr(id)}" ${id ? "" : "disabled"}>Approve</button>
        <button class="button danger" data-action="reject-memory" data-candidate-id="${escapeAttr(id)}" ${id ? "" : "disabled"}>Reject</button>
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
          <button class="button secondary" data-action="load-admin">Reload</button>
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
          <button class="button secondary" data-action="load-evaluation">Reload</button>
        </div>
        <form id="evaluation-form" class="evaluation-form">
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
          <button class="button primary" type="submit">Save case</button>
        </form>
        ${state.evaluation.cases.map((item) => `
          <div class="case-row">
            <button class="list-item ${state.evaluation.selectedCaseId === item.case_id ? "active" : ""}" data-eval-case-id="${escapeAttr(item.case_id)}">
              <strong>${escapeHtml(item.case_id)}</strong>
              <span>${escapeHtml(item.suite)} · ${escapeHtml(item.task_type)}</span>
            </button>
            <button class="button secondary" data-action="run-evaluation" data-case-id="${escapeAttr(item.case_id)}">Run dry</button>
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

  document.querySelector<HTMLButtonElement>("[data-action='logout']")?.addEventListener("click", () => void handleLogout());
  document.querySelector<HTMLButtonElement>("[data-action='refresh']")?.addEventListener("click", () => void refreshCurrentView());
  document.querySelector<HTMLFormElement>("#session-form")?.addEventListener("submit", onSessionSubmit);
  document.querySelector<HTMLFormElement>("#answer-form")?.addEventListener("submit", onAnswerSubmit);
  document.querySelector<HTMLButtonElement>("[data-action='poll-session']")?.addEventListener("click", () => void pollSession());
  document.querySelector<HTMLButtonElement>("[data-action='load-trace']")?.addEventListener("click", () => void loadTrace());
  document.querySelector<HTMLButtonElement>("[data-action='generate-report']")?.addEventListener("click", () => void generateReport());
  document.querySelector<HTMLButtonElement>("[data-action='finalize-session']")?.addEventListener("click", () => void finalizeSession());
  document.querySelector<HTMLButtonElement>("[data-action='load-coding']")?.addEventListener("click", () => void loadCoding());
  document.querySelector<HTMLFormElement>("#coding-form")?.addEventListener("submit", onCodingSubmit);
  document.querySelectorAll<HTMLButtonElement>("[data-question-id]").forEach((button) => {
    button.addEventListener("click", () => void selectQuestion(button.dataset.questionId ?? ""));
  });
  document.querySelector<HTMLButtonElement>("[data-action='load-memory']")?.addEventListener("click", () => void loadMemory());
  document.querySelectorAll<HTMLButtonElement>("[data-action='approve-memory']").forEach((button) => {
    button.addEventListener("click", () => void reviewMemory(button.dataset.candidateId ?? "", "approve"));
  });
  document.querySelectorAll<HTMLButtonElement>("[data-action='reject-memory']").forEach((button) => {
    button.addEventListener("click", () => void reviewMemory(button.dataset.candidateId ?? "", "reject"));
  });
  document.querySelector<HTMLButtonElement>("[data-action='load-admin']")?.addEventListener("click", () => void loadAdmin());
  document.querySelector<HTMLButtonElement>("[data-action='load-evaluation']")?.addEventListener("click", () => void loadEvaluation());
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
  try {
    const auth = await api.login(email, password);
    saveAuth(auth);
    await loadInitialData();
    toast("Signed in");
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
  const form = new FormData(event.currentTarget as HTMLFormElement);
  state.interview.loading = true;
  state.interview.error = "";
  render();
  try {
    state.interview.session = await api.createInterviewSession(
      String(form.get("skill_id") ?? "java-backend"),
      String(form.get("question_type") ?? "backend"),
      Number(form.get("max_follow_ups") ?? 1)
    );
    toast("Session created");
  } catch (error) {
    state.interview.error = errorMessage(error);
  }
  state.interview.loading = false;
  render();
}

async function onAnswerSubmit(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  if (!state.interview.session) return;
  const form = new FormData(event.currentTarget as HTMLFormElement);
  state.interview.answer = String(form.get("answer") ?? "");
  state.interview.dryRun = form.get("dry_run") === "on";
  try {
    await api.submitInterviewAnswer(state.interview.session, state.interview.answer, state.interview.dryRun);
    toast("Answer accepted by API");
    await pollSession();
  } catch (error) {
    state.interview.error = errorMessage(error);
    render();
  }
}

async function pollSession(): Promise<void> {
  if (!state.interview.session) return;
  try {
    state.interview.session = await api.getInterviewSession(state.interview.session.session_id);
  } catch (error) {
    state.interview.error = errorMessage(error);
  }
  render();
}

async function loadTrace(): Promise<void> {
  if (!state.interview.session) return;
  try {
    state.interview.trace = await api.getInterviewTrace(state.interview.session.session_id);
  } catch (error) {
    state.interview.error = errorMessage(error);
  }
  render();
}

async function generateReport(): Promise<void> {
  if (!state.interview.session) return;
  try {
    state.interview.report = await api.generateInterviewReport(state.interview.session.session_id, true);
  } catch (error) {
    state.interview.error = errorMessage(error);
  }
  render();
}

async function finalizeSession(): Promise<void> {
  if (!state.interview.session) return;
  try {
    state.interview.session = await api.finalizeInterviewSession(state.interview.session.session_id);
  } catch (error) {
    state.interview.error = errorMessage(error);
  }
  render();
}

async function onCodingSubmit(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  if (!state.coding.selectedQuestion) return;
  const form = new FormData(event.currentTarget as HTMLFormElement);
  state.coding.language = String(form.get("language") ?? "go");
  state.coding.sourceCode = String(form.get("source_code") ?? "");
  try {
    await api.createCodingSubmission(state.coding.selectedQuestion.question_id, state.coding.language, state.coding.sourceCode);
    toast("Submission queued");
    state.coding.submissions = await api.listCodingSubmissions(state.coding.selectedQuestion.question_id);
  } catch (error) {
    state.coding.error = errorMessage(error);
  }
  render();
}

async function reviewMemory(candidateId: string, action: "approve" | "reject"): Promise<void> {
  if (!candidateId) return;
  try {
    if (action === "approve") await api.approveMemoryCandidate(candidateId, "Approved from frontend review");
    else await api.rejectMemoryCandidate(candidateId, "Rejected from frontend review");
    toast(`Memory ${action}d`);
    await loadMemory();
  } catch (error) {
    state.memory.error = errorMessage(error);
    render();
  }
}

async function onEvaluationSubmit(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  const form = new FormData(event.currentTarget as HTMLFormElement);
  try {
    await api.saveEvaluationCase({
      case_id: String(form.get("case_id") ?? ""),
      suite: String(form.get("suite") ?? "runtime-smoke"),
      task_type: String(form.get("task_type") ?? "question_generation"),
      skill_id: String(form.get("skill_id") ?? "java-backend"),
      input: { user_input: String(form.get("user_input") ?? "") },
      expected: {
        required_fields: ["question"],
        contains: { question: "Redis" }
      },
      tags: ["frontend", "smoke"],
      status: "active"
    });
    toast("Evaluation case saved");
    await loadEvaluation();
  } catch (error) {
    state.evaluation.error = errorMessage(error);
    render();
  }
}

async function runEvaluation(caseId: string): Promise<void> {
  try {
    state.evaluation.lastRun = await api.runEvaluationCase(caseId, true);
    toast("Evaluation run recorded");
    await loadEvaluation();
  } catch (error) {
    state.evaluation.error = errorMessage(error);
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

function skillOptions(selected = ""): string {
  const items = state.skills.length ? state.skills : [{ id: "java-backend", display_name: "Java backend" }];
  return items.map((skill) => {
    const id = skill.id ?? skill.skill_id ?? "java-backend";
    const label = skill.display_name ?? id;
    return `<option value="${escapeAttr(id)}" ${id === selected ? "selected" : ""}>${escapeHtml(label)}</option>`;
  }).join("");
}

function metricCard(label: string, value: string, hint: string, tone: string): string {
  return `
    <article class="metric-card ${tone}">
      <span>${escapeHtml(label)}</span>
      <strong>${escapeHtml(value)}</strong>
      <small>${escapeHtml(hint)}</small>
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

function emptyState(title: string, copy: string): string {
  return `<div class="empty-state"><strong>${escapeHtml(title)}</strong><p>${escapeHtml(copy)}</p></div>`;
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
