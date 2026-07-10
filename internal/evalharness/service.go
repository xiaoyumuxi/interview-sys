package evalharness

import (
	"context"
	"time"

	"ai-interview-platform/internal/contextengine"
	airuntime "ai-interview-platform/internal/runtime"
	corestore "ai-interview-platform/internal/store"
)

type Service struct {
	store         *Store
	coreStore     *corestore.Store
	contextEngine *contextengine.Engine
	runtimeClient *airuntime.Client
}

func NewService(store *Store, coreStore *corestore.Store, contextEngine *contextengine.Engine, runtimeClient *airuntime.Client) *Service {
	return &Service{
		store:         store,
		coreStore:     coreStore,
		contextEngine: contextEngine,
		runtimeClient: runtimeClient,
	}
}

type RunCaseRequest struct {
	CaseID string `json:"case_id"`
	UserID string `json:"user_id,omitempty"`
	DryRun bool   `json:"dry_run"`
}

type RunResult struct {
	SchemaVersion   string                         `json:"schema_version"`
	Case            Case                           `json:"case"`
	Run             Run                            `json:"run"`
	ContextPreview  *contextengine.PreviewResponse `json:"context_preview,omitempty"`
	RuntimeResponse *airuntime.TaskResponse        `json:"runtime_response,omitempty"`
}

func (s *Service) SaveCase(ctx context.Context, req SaveCaseRequest) (Case, error) {
	return s.store.SaveCase(ctx, req)
}

func (s *Service) GetCase(ctx context.Context, caseID string) (Case, bool, error) {
	return s.store.GetCase(ctx, caseID)
}

func (s *Service) ListCases(ctx context.Context, suite string, taskType string, limit int) ([]Case, error) {
	return s.store.ListCases(ctx, suite, taskType, limit)
}

func (s *Service) ListRuns(ctx context.Context, caseID string, taskType string, limit int) ([]Run, error) {
	return s.store.ListRuns(ctx, caseID, taskType, limit)
}

func (s *Service) RunCase(ctx context.Context, req RunCaseRequest) (RunResult, error) {
	started := time.Now()
	item, ok, err := s.store.GetCase(ctx, req.CaseID)
	if err != nil {
		return RunResult{}, err
	}
	if !ok {
		return RunResult{}, ErrCaseNotFound
	}

	userInput := stringFromMap(item.Input, "user_input")
	if userInput == "" {
		userInput = compactJSON(item.Input)
	}
	memoryQuery := stringFromMap(item.Input, "memory_query")
	if memoryQuery == "" {
		memoryQuery = userInput
	}
	userID := req.UserID
	if caseUserID := stringFromMap(item.Input, "user_id"); caseUserID != "" {
		userID = caseUserID
	}

	preview, err := s.contextEngine.Preview(ctx, contextengine.PreviewRequest{
		TaskType:    item.TaskType,
		SkillID:     item.SkillID,
		UserID:      userID,
		MemoryQuery: memoryQuery,
		ResumeID:    stringFromMap(item.Input, "resume_id"),
		JDID:        stringFromMap(item.Input, "jd_id"),
		SessionID:   stringFromMap(item.Input, "session_id"),
	})
	if err != nil {
		run, recordErr := s.recordError(ctx, item, req, started, "context_preview_failed: "+err.Error())
		if recordErr != nil {
			return RunResult{}, recordErr
		}
		return RunResult{SchemaVersion: "evaluation.run.v1", Case: item, Run: run}, nil
	}

	provider, err := s.coreStore.RuntimeProviderForTask(ctx, item.TaskType)
	if err != nil {
		run, recordErr := s.recordError(ctx, item, req, started, "provider_resolution_failed: "+err.Error())
		if recordErr != nil {
			return RunResult{}, recordErr
		}
		return RunResult{SchemaVersion: "evaluation.run.v1", Case: item, Run: run, ContextPreview: &preview}, nil
	}

	runtimeResp, err := s.runtimeClient.RunTask(ctx, airuntime.TaskRequest{
		TaskType:     item.TaskType,
		Provider:     provider,
		ContextItems: preview.Items,
		UserInput:    userInput,
		OutputSchema: outputSchema(item.Expected),
		DryRun:       req.DryRun,
	})
	if err != nil {
		run, recordErr := s.recordError(ctx, item, req, started, "runtime_task_failed: "+err.Error())
		if recordErr != nil {
			return RunResult{}, recordErr
		}
		return RunResult{SchemaVersion: "evaluation.run.v1", Case: item, Run: run, ContextPreview: &preview}, nil
	}

	traceID := corestore.NewID("trace")
	if err := s.coreStore.InsertAgentTrace(ctx, corestore.TraceRecord{
		TraceID:      traceID,
		TaskType:     item.TaskType,
		SkillID:      item.SkillID,
		Input:        map[string]any{"case": item, "dry_run": req.DryRun},
		ContextItems: preview.Items,
		Output:       runtimeResp,
	}); err != nil {
		traceID = ""
	}

	assertions := EvaluateAssertions(runtimeResp.Output, item.Expected)
	status, score := StatusAndScore(assertions)
	output := map[string]any{
		"runtime_response": runtimeResp,
	}
	run, err := s.store.RecordRun(ctx, RecordRunRequest{
		CaseID:     item.CaseID,
		TaskType:   item.TaskType,
		Status:     status,
		Score:      score,
		Input:      runInput(item, req),
		Output:     output,
		Assertions: assertions,
		TraceID:    traceID,
		DurationMS: int(time.Since(started).Milliseconds()),
	})
	if err != nil {
		return RunResult{}, err
	}
	return RunResult{
		SchemaVersion:   "evaluation.run.v1",
		Case:            item,
		Run:             run,
		ContextPreview:  &preview,
		RuntimeResponse: &runtimeResp,
	}, nil
}

func (s *Service) recordError(ctx context.Context, item Case, req RunCaseRequest, started time.Time, message string) (Run, error) {
	return s.store.RecordRun(ctx, RecordRunRequest{
		CaseID:     item.CaseID,
		TaskType:   item.TaskType,
		Status:     "error",
		Score:      0,
		Input:      runInput(item, req),
		Output:     map[string]any{},
		Assertions: []Assertion{},
		ErrorText:  message,
		DurationMS: int(time.Since(started).Milliseconds()),
	})
}

func runInput(item Case, req RunCaseRequest) map[string]any {
	return map[string]any{
		"case_id": item.CaseID,
		"suite":   item.Suite,
		"input":   item.Input,
		"dry_run": req.DryRun,
		"user_id": req.UserID,
	}
}

func stringFromMap(input map[string]any, key string) string {
	value, ok := input[key]
	if !ok || value == nil {
		return ""
	}
	return defaultString(compactString(value), "")
}

func compactString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return compactJSON(value)
}

func outputSchema(expected map[string]any) map[string]any {
	schema, ok := mapFromAny(expected["output_schema"])
	if !ok {
		return nil
	}
	return schema
}
