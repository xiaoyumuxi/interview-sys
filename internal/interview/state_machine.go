package interview

import "fmt"

var sessionTransitions = map[string]map[string]bool{
	SessionReady: {
		SessionInProgress: true,
		SessionFinished:   true,
		SessionFailed:     true,
	},
	SessionInProgress: {
		SessionFinished: true,
		SessionFailed:   true,
	},
	SessionFinished: {},
	SessionFailed:   {},
}

var flowTransitions = map[string]map[string]bool{
	FlowInit: {
		FlowAsking:    true,
		FlowCompleted: true,
	},
	FlowAsking: {
		FlowEvaluating: true,
		FlowFollowUp:   true,
		FlowCompleted:  true,
	},
	FlowEvaluating: {
		FlowAsking:    true,
		FlowFollowUp:  true,
		FlowCompleted: true,
	},
	FlowFollowUp: {
		FlowEvaluating: true,
		FlowAsking:     true,
		FlowCompleted:  true,
	},
	FlowCompleted: {},
}

var turnTransitions = map[string]map[string]bool{
	TurnQueued: {
		TurnRunning:   true,
		TurnCompleted: true,
		TurnFailed:    true,
	},
	TurnRunning: {
		TurnQueued:    true,
		TurnCompleted: true,
		TurnFailed:    true,
	},
	TurnCompleted: {},
	TurnFailed:    {},
}

func validateSessionTransition(from, to string) error {
	if from == to {
		return nil
	}
	next := sessionTransitions[from]
	if next == nil || !next[to] {
		return fmt.Errorf("invalid session transition: %s -> %s", from, to)
	}
	return nil
}

func validateFlowTransition(from, to string) error {
	if from == to {
		return nil
	}
	next := flowTransitions[from]
	if next == nil || !next[to] {
		return fmt.Errorf("invalid flow transition: %s -> %s", from, to)
	}
	return nil
}

func validateTurnTransition(from, to string) error {
	if from == to {
		return nil
	}
	next := turnTransitions[from]
	if next == nil || !next[to] {
		return fmt.Errorf("invalid turn transition: %s -> %s", from, to)
	}
	return nil
}
