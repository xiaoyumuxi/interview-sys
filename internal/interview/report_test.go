package interview

import "testing"

func TestBuildReportInputAggregatesTurnFacts(t *testing.T) {
	session := Session{
		SessionID:     "sess_1",
		UserID:        "user_1",
		SkillID:       "java-backend",
		SessionStatus: SessionFinished,
		FlowStatus:    FlowCompleted,
		TotalScore:    16,
		MaxFollowUps:  1,
		Metadata:      map[string]any{"question_type": "algorithm"},
		CreatedAt:     "2026-07-06T10:00:00Z",
		FinishedAt:    "2026-07-06T10:10:00Z",
		Turns: []Turn{
			{
				TurnID:         "turn_1",
				QuestionID:     "q_1",
				QuestionNumber: 1,
				AnswerRound:    0,
				TurnStatus:     TurnCompleted,
				UserAnswer:     "Use a hash map.",
				Score:          8,
				Evaluation: map[string]any{
					"strengths":  []any{"clear complexity"},
					"weaknesses": []any{"missed edge case"},
					"evidence":   []any{"explained O(n)"},
				},
			},
			{
				TurnID:         "turn_2",
				QuestionID:     "q_2",
				QuestionNumber: 2,
				AnswerRound:    0,
				TurnStatus:     TurnCompleted,
				UserAnswer:     "Discuss Redis Stream retries.",
				Score:          6,
				Evaluation: map[string]any{
					"strengths": []any{"understands pending reclaim"},
				},
			},
			{
				TurnID:     "turn_3",
				TurnStatus: TurnFailed,
				ErrorText:  "runtime timeout",
				Evaluation: map[string]any{},
			},
		},
	}

	input := buildReportInput(session)
	if input["schema_version"] != "interview.report.input.v1" {
		t.Fatalf("schema_version = %v", input["schema_version"])
	}
	sessionFacts := input["session"].(map[string]any)
	if sessionFacts["completed_turns"] != 2 {
		t.Fatalf("completed_turns = %v", sessionFacts["completed_turns"])
	}
	if sessionFacts["failed_turns"] != 1 {
		t.Fatalf("failed_turns = %v", sessionFacts["failed_turns"])
	}
	if sessionFacts["average_score"] != 7.0 {
		t.Fatalf("average_score = %v", sessionFacts["average_score"])
	}
	if len(sessionFacts["strengths"].([]any)) != 2 {
		t.Fatalf("strengths = %#v", sessionFacts["strengths"])
	}
	if len(sessionFacts["weaknesses"].([]any)) != 1 {
		t.Fatalf("weaknesses = %#v", sessionFacts["weaknesses"])
	}
	turns := input["turns"].([]map[string]any)
	if len(turns) != 3 {
		t.Fatalf("turn count = %d", len(turns))
	}
	if turns[2]["error_text"] != "runtime timeout" {
		t.Fatalf("failed turn error = %v", turns[2]["error_text"])
	}
}

func TestFinalReportOutputSchemaHasStableVersion(t *testing.T) {
	schema := finalReportOutputSchema()
	if schema["schema_version"] != "interview.report.content.v1" {
		t.Fatalf("schema_version = %v", schema["schema_version"])
	}
	if _, ok := schema["review_plan"]; !ok {
		t.Fatalf("schema missing review_plan: %#v", schema)
	}
}
