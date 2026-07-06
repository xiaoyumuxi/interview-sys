package coding

import (
	"context"
	"log/slog"
	"time"
)

type Evaluator interface {
	Evaluate(ctx context.Context, submission Submission, question Question) (JudgeResult, error)
}

type Worker struct {
	store     *Store
	evaluator Evaluator
	logger    *slog.Logger
}

type WorkerOptions struct {
	BatchSize int
	Interval  time.Duration
}

func NewWorker(store *Store, evaluator Evaluator, logger *slog.Logger) *Worker {
	if evaluator == nil {
		evaluator = SandboxUnavailableEvaluator{}
	}
	return &Worker{store: store, evaluator: evaluator, logger: logger}
}

func DefaultWorkerOptions(batchSize int) WorkerOptions {
	if batchSize <= 0 {
		batchSize = 4
	}
	return WorkerOptions{
		BatchSize: batchSize,
		Interval:  2 * time.Second,
	}
}

func (w *Worker) Start(ctx context.Context, opts WorkerOptions) {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 4
	}
	if opts.Interval <= 0 {
		opts.Interval = 2 * time.Second
	}
	go func() {
		ticker := time.NewTicker(opts.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := w.ProcessBatch(ctx, opts.BatchSize); err != nil && w.logger != nil {
					w.logger.Warn("coding judge batch failed", "error", err)
				}
			}
		}
	}()
}

func (w *Worker) ProcessBatch(ctx context.Context, batchSize int) (int, error) {
	submissions, err := w.store.ClaimQueuedSubmissions(ctx, batchSize)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, submission := range submissions {
		if err := w.processSubmission(ctx, submission); err != nil {
			if w.logger != nil {
				w.logger.Warn("coding judge submission failed", "submission_id", submission.SubmissionID, "error", err)
			}
			continue
		}
		processed++
	}
	return processed, nil
}

func (w *Worker) processSubmission(ctx context.Context, submission Submission) error {
	question, ok, err := w.store.GetQuestion(ctx, submission.QuestionID)
	if err != nil {
		return err
	}
	if !ok {
		_, err := w.store.CompleteSubmission(ctx, submission.SubmissionID, JudgeResult{
			Status: StatusSystemError,
			Result: map[string]any{
				"error_code": "question_not_found",
				"message":    "question was not found while judging submission",
				"retryable":  false,
			},
		})
		return err
	}
	result, err := w.evaluator.Evaluate(ctx, submission, question)
	if err != nil {
		result = JudgeResult{
			Status: StatusSystemError,
			Result: map[string]any{
				"error_code": "judge_evaluator_failed",
				"message":    err.Error(),
				"retryable":  true,
			},
			StderrText: err.Error(),
		}
	}
	_, err = w.store.CompleteSubmission(ctx, submission.SubmissionID, result)
	return err
}

type SandboxUnavailableEvaluator struct{}

func (SandboxUnavailableEvaluator) Evaluate(ctx context.Context, submission Submission, question Question) (JudgeResult, error) {
	_ = ctx
	_ = question
	return JudgeResult{
		Status: StatusSystemError,
		Score:  0,
		Result: map[string]any{
			"error_code":    "sandbox_not_configured",
			"message":       "coding judge sandbox is not configured; no user code was executed",
			"retryable":     false,
			"language":      submission.Language,
			"submission_id": submission.SubmissionID,
		},
		TestResults: []map[string]any{},
		ResourceUsage: map[string]any{
			"sandbox": "unavailable",
		},
	}, nil
}
