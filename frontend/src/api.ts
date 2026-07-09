export type JsonObject = Record<string, unknown>;

export interface User {
  user_id: string;
  display_name: string;
  email?: string;
  role: "root" | "user" | string;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_in: number;
  refresh_expires_in: number;
}

export interface Skill {
  id?: string;
  skill_id?: string;
  display_name?: string;
  description?: string;
  meta?: JsonObject;
}

export interface InterviewQuestion {
  question_id: string;
  number: number;
  title: string;
  prompt: string;
  tags?: string[];
}

export interface InterviewTurn {
  turn_id: string;
  question_id?: string;
  question_number: number;
  answer_round: number;
  request_id: string;
  user_answer: string;
  evaluation: JsonObject;
  follow_up_needed: boolean;
  follow_up_question?: string;
  score: number;
  trace_id?: string;
  response: JsonObject;
  turn_status: string;
  error_text?: string;
  created_at: string;
  updated_at: string;
}

export interface InterviewSession {
  session_id: string;
  user_id: string;
  skill_id: string;
  session_status: string;
  flow_status: string;
  phase: string;
  current_question_id?: string;
  current_question_number: number;
  answer_round: number;
  follow_up_count: number;
  max_follow_ups: number;
  total_score: number;
  metadata: JsonObject;
  last_error?: string;
  created_at: string;
  updated_at: string;
  finished_at?: string;
  current_question?: InterviewQuestion;
  turns?: InterviewTurn[];
}

export interface CodingQuestion {
  question_id: string;
  title: string;
  difficulty: string;
  question_type: string;
  topic_tags?: string[];
  company_tags?: string[];
  prompt?: string;
  input_format?: string;
  output_format?: string;
  constraints_text?: string;
}

export interface CodingSubmission {
  submission_id: string;
  question_id: string;
  language: string;
  status: string;
  score: number;
  result: JsonObject;
  created_at: string;
  updated_at: string;
}

export interface CodingCompletionSuggestion {
  id: string;
  label: string;
  detail: string;
  insert_text: string;
  kind: string;
  source: string;
  rank: number;
  tags: string[];
}

export interface CodingCompletionResponse {
  schema_version: string;
  question_id?: string;
  language: string;
  prefix: string;
  cursor_offset: number;
  capabilities: string[];
  suggestions: CodingCompletionSuggestion[];
  diagnostics: Array<{ code: string; message: string }>;
}

export interface EvaluationCase {
  case_id: string;
  suite: string;
  task_type: string;
  skill_id?: string;
  input: JsonObject;
  expected: JsonObject;
  tags: string[];
  status: string;
  created_at: string;
  updated_at: string;
}

export interface EvaluationRun {
  run_id: string;
  case_id: string;
  task_type: string;
  status: string;
  score: number;
  input: JsonObject;
  output: JsonObject;
  assertions: JsonObject[];
  trace_id?: string;
  error_text?: string;
  duration_ms: number;
  created_at: string;
}

export interface ApiState {
  accessToken: string;
  refreshToken: string;
  user: User;
}

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export class ApiClient {
  private accessToken = "";
  private refreshToken = "";

  setTokens(accessToken: string, refreshToken: string): void {
    this.accessToken = accessToken;
    this.refreshToken = refreshToken;
  }

  clearTokens(): void {
    this.accessToken = "";
    this.refreshToken = "";
  }

  async health(): Promise<JsonObject> {
    return this.request<JsonObject>("/healthz", { auth: false });
  }

  async login(email: string, password: string): Promise<ApiState> {
    const response = await this.request<{ user: User; tokens: TokenPair }>("/api/auth/login", {
      method: "POST",
      auth: false,
      body: { email, password }
    });
    this.setTokens(response.tokens.access_token, response.tokens.refresh_token);
    return {
      user: response.user,
      accessToken: response.tokens.access_token,
      refreshToken: response.tokens.refresh_token
    };
  }

  async me(): Promise<User> {
    const response = await this.request<{ user: User }>("/api/auth/me");
    return response.user;
  }

  async logout(): Promise<void> {
    if (!this.refreshToken) return;
    await this.request("/api/auth/logout", {
      method: "POST",
      body: { refresh_token: this.refreshToken }
    });
  }

  async listSkills(): Promise<Skill[]> {
    return this.items<Skill>("/api/skills");
  }

  async createInterviewSession(skillId: string, questionType: string, maxFollowUps: number): Promise<InterviewSession> {
    const response = await this.request<{ item: InterviewSession }>("/api/interview-sessions", {
      method: "POST",
      body: {
        skill_id: skillId,
        question_type: questionType,
        max_follow_ups: maxFollowUps,
        metadata: { source: "frontend" }
      }
    });
    return response.item;
  }

  async getInterviewSession(sessionId: string): Promise<InterviewSession> {
    const response = await this.request<{ item: InterviewSession }>(`/api/interview-sessions/${encodeURIComponent(sessionId)}`);
    return response.item;
  }

  async submitInterviewAnswer(session: InterviewSession, answer: string, dryRun: boolean): Promise<JsonObject> {
    return this.request<JsonObject>(`/api/interview-sessions/${encodeURIComponent(session.session_id)}/answers`, {
      method: "POST",
      body: {
        request_id: `frontend-${Date.now()}`,
        question_id: session.current_question_id ?? session.current_question?.question_id ?? "",
        question_number: session.current_question_number || 1,
        answer_round: session.answer_round || 0,
        user_answer: answer,
        dry_run: dryRun
      }
    });
  }

  async finalizeInterviewSession(sessionId: string): Promise<InterviewSession> {
    const response = await this.request<{ item: InterviewSession }>(`/api/interview-sessions/${encodeURIComponent(sessionId)}/finalize`, {
      method: "POST",
      body: {}
    });
    return response.item;
  }

  async getInterviewTrace(sessionId: string): Promise<JsonObject[]> {
    return this.items<JsonObject>(`/api/interview-sessions/${encodeURIComponent(sessionId)}/trace`);
  }

  async generateInterviewReport(sessionId: string, dryRun: boolean): Promise<JsonObject> {
    const response = await this.request<{ item: JsonObject }>(`/api/interview-sessions/${encodeURIComponent(sessionId)}/report`, {
      method: "POST",
      body: { dry_run: dryRun }
    });
    return response.item;
  }

  async listCodingQuestions(): Promise<CodingQuestion[]> {
    return this.items<CodingQuestion>("/api/coding/questions?limit=30");
  }

  async getCodingQuestion(questionId: string): Promise<CodingQuestion> {
    return this.request<CodingQuestion>(`/api/coding/questions/${encodeURIComponent(questionId)}`);
  }

  async createCodingSubmission(questionId: string, language: string, sourceCode: string): Promise<CodingSubmission> {
    const response = await this.request<{ item: CodingSubmission }>("/api/coding/submissions", {
      method: "POST",
      body: { question_id: questionId, language, source_code: sourceCode }
    });
    return response.item;
  }

  async listCodingSubmissions(questionId = ""): Promise<CodingSubmission[]> {
    const query = questionId ? `?question_id=${encodeURIComponent(questionId)}&limit=20` : "?limit=20";
    return this.items<CodingSubmission>(`/api/coding/submissions${query}`);
  }

  async suggestCodingCompletions(payload: {
    question_id?: string;
    language: string;
    source_code: string;
    cursor_offset: number;
    prefix?: string;
    limit?: number;
  }): Promise<CodingCompletionResponse> {
    return this.request<CodingCompletionResponse>("/api/coding/completions", {
      method: "POST",
      body: payload
    });
  }

  async listMemoryCandidates(status = "pending"): Promise<JsonObject> {
    return this.request<JsonObject>(`/api/memory/candidates?status=${encodeURIComponent(status)}&limit=50`);
  }

  async approveMemoryCandidate(candidateId: string, reviewNote: string): Promise<JsonObject> {
    return this.request<JsonObject>(`/api/memory/candidates/${encodeURIComponent(candidateId)}/approve`, {
      method: "POST",
      body: { review_note: reviewNote }
    });
  }

  async rejectMemoryCandidate(candidateId: string, reviewNote: string): Promise<JsonObject> {
    return this.request<JsonObject>(`/api/memory/candidates/${encodeURIComponent(candidateId)}/reject`, {
      method: "POST",
      body: { review_note: reviewNote }
    });
  }

  async listProviders(): Promise<JsonObject[]> {
    return this.items<JsonObject>("/api/providers");
  }

  async listProviderRoutes(): Promise<JsonObject[]> {
    return this.items<JsonObject>("/api/provider-routes");
  }

  async workerSummary(): Promise<JsonObject> {
    return this.request<JsonObject>("/api/ops/workers/summary");
  }

  async codingJudgeSummary(): Promise<JsonObject> {
    return this.request<JsonObject>("/api/ops/coding-judge/summary");
  }

  async listEvaluationCases(): Promise<EvaluationCase[]> {
    return this.items<EvaluationCase>("/api/evaluation/cases?limit=50");
  }

  async saveEvaluationCase(payload: Partial<EvaluationCase>): Promise<EvaluationCase> {
    const response = await this.request<{ item: EvaluationCase }>("/api/evaluation/cases", {
      method: "POST",
      body: payload
    });
    return response.item;
  }

  async runEvaluationCase(caseId: string, dryRun: boolean): Promise<JsonObject> {
    return this.request<JsonObject>(`/api/evaluation/cases/${encodeURIComponent(caseId)}/run`, {
      method: "POST",
      body: { dry_run: dryRun }
    });
  }

  async listEvaluationRuns(): Promise<EvaluationRun[]> {
    return this.items<EvaluationRun>("/api/evaluation/runs?limit=20");
  }

  async retrievalSearch(payload: JsonObject): Promise<JsonObject> {
    return this.request<JsonObject>("/api/retrieval/search", {
      method: "POST",
      body: payload
    });
  }

  private async items<T>(path: string): Promise<T[]> {
    const response = await this.request<{ items?: T[] }>(path);
    return response.items ?? [];
  }

  private async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const headers = new Headers();
    if (options.body !== undefined) headers.set("Content-Type", "application/json");
    if (options.auth !== false && this.accessToken) headers.set("Authorization", `Bearer ${this.accessToken}`);

    const response = await fetch(path, {
      method: options.method ?? "GET",
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body)
    });

    const text = await response.text();
    const data = text ? (JSON.parse(text) as unknown) : {};
    if (!response.ok) {
      const error = readError(data);
      throw new ApiError(response.status, error.code, error.message);
    }
    return data as T;
  }
}

interface RequestOptions {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
  auth?: boolean;
}

function readError(value: unknown): { code: string; message: string } {
  if (isRecord(value)) {
    const error = value.error;
    if (isRecord(error)) {
      return {
        code: typeof error.code === "string" ? error.code : "request_failed",
        message: typeof error.message === "string" ? error.message : "Request failed"
      };
    }
  }
  return { code: "request_failed", message: "Request failed" };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
