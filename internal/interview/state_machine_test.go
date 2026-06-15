package interview

import "testing"

func TestValidateSessionTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{name: "ready to in progress", from: SessionReady, to: SessionInProgress},
		{name: "ready to finished", from: SessionReady, to: SessionFinished},
		{name: "in progress to failed", from: SessionInProgress, to: SessionFailed},
		{name: "finished is terminal", from: SessionFinished, to: SessionInProgress, wantErr: true},
		{name: "failed is terminal", from: SessionFailed, to: SessionFinished, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateSessionTransition(test.from, test.to)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateSessionTransition(%q, %q) error = %v, wantErr %v", test.from, test.to, err, test.wantErr)
			}
		})
	}
}

func TestValidateFlowTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{name: "init to asking", from: FlowInit, to: FlowAsking},
		{name: "asking to evaluating", from: FlowAsking, to: FlowEvaluating},
		{name: "evaluating to follow up", from: FlowEvaluating, to: FlowFollowUp},
		{name: "follow up to evaluating", from: FlowFollowUp, to: FlowEvaluating},
		{name: "completed is terminal", from: FlowCompleted, to: FlowAsking, wantErr: true},
		{name: "init cannot evaluate", from: FlowInit, to: FlowEvaluating, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateFlowTransition(test.from, test.to)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateFlowTransition(%q, %q) error = %v, wantErr %v", test.from, test.to, err, test.wantErr)
			}
		})
	}
}

func TestValidateTurnTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{name: "queued to running", from: TurnQueued, to: TurnRunning},
		{name: "running to queued reclaim", from: TurnRunning, to: TurnQueued},
		{name: "running to completed", from: TurnRunning, to: TurnCompleted},
		{name: "queued to failed", from: TurnQueued, to: TurnFailed},
		{name: "completed is terminal", from: TurnCompleted, to: TurnRunning, wantErr: true},
		{name: "failed is terminal", from: TurnFailed, to: TurnQueued, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateTurnTransition(test.from, test.to)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateTurnTransition(%q, %q) error = %v, wantErr %v", test.from, test.to, err, test.wantErr)
			}
		})
	}
}
