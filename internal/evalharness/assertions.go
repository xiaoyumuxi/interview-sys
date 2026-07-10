package evalharness

import (
	"fmt"
	"reflect"
	"strings"
)

type Assertion struct {
	Path     string `json:"path"`
	Operator string `json:"operator"`
	Expected any    `json:"expected,omitempty"`
	Actual   any    `json:"actual,omitempty"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
}

func EvaluateAssertions(output map[string]any, expected map[string]any) []Assertion {
	assertions := make([]Assertion, 0)
	for _, path := range stringSlice(expected["required_fields"]) {
		actual, ok := valueAtPath(output, path)
		assertions = append(assertions, Assertion{
			Path:     path,
			Operator: "required",
			Actual:   actual,
			Passed:   ok && !isEmptyValue(actual),
			Message:  assertionMessage(ok && !isEmptyValue(actual), "field is present", "field is missing or empty"),
		})
	}
	if contains, ok := mapFromAny(expected["contains"]); ok {
		for path, expectedValue := range contains {
			actual, ok := valueAtPath(output, path)
			expectedText := fmt.Sprint(expectedValue)
			actualText := fmt.Sprint(actual)
			passed := ok && strings.Contains(actualText, expectedText)
			assertions = append(assertions, Assertion{
				Path:     path,
				Operator: "contains",
				Expected: expectedValue,
				Actual:   actual,
				Passed:   passed,
				Message:  assertionMessage(passed, "text contains expected value", "text does not contain expected value"),
			})
		}
	}
	if equals, ok := mapFromAny(expected["equals"]); ok {
		for path, expectedValue := range equals {
			actual, ok := valueAtPath(output, path)
			passed := ok && (reflect.DeepEqual(actual, expectedValue) || fmt.Sprint(actual) == fmt.Sprint(expectedValue))
			assertions = append(assertions, Assertion{
				Path:     path,
				Operator: "equals",
				Expected: expectedValue,
				Actual:   actual,
				Passed:   passed,
				Message:  assertionMessage(passed, "value equals expected value", "value does not equal expected value"),
			})
		}
	}
	if len(assertions) == 0 {
		assertions = append(assertions, Assertion{
			Path:     "$",
			Operator: "runtime_ok",
			Passed:   true,
			Message:  "no expected assertions configured",
		})
	}
	return assertions
}

func StatusAndScore(assertions []Assertion) (string, float64) {
	if len(assertions) == 0 {
		return "passed", 100
	}
	passed := 0
	for _, assertion := range assertions {
		if assertion.Passed {
			passed++
		}
	}
	score := float64(passed) / float64(len(assertions)) * 100
	if passed == len(assertions) {
		return "passed", score
	}
	return "failed", score
}

func valueAtPath(output map[string]any, path string) (any, bool) {
	path = strings.TrimSpace(path)
	if path == "" || path == "$" {
		return output, true
	}
	var current any = output
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = currentMap[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func isEmptyValue(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

func assertionMessage(passed bool, success string, failure string) string {
	if passed {
		return success
	}
	return failure
}
