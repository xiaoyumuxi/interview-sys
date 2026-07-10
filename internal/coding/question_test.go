package coding

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQuestionCandidateViewRemovesAuthoringMaterial(t *testing.T) {
	original := Question{
		QuestionID:        "question_1",
		Title:             "Two Sum",
		Prompt:            "Return the matching indices.",
		ReferenceSolution: "secret solution",
		Explanation:       "secret explanation",
		Status:            "published",
	}

	view := original.CandidateView()
	if view.ReferenceSolution != "" || view.Explanation != "" {
		t.Fatalf("CandidateView() leaked authoring material: %+v", view)
	}
	if original.ReferenceSolution == "" || original.Explanation == "" {
		t.Fatal("CandidateView() mutated the stored question value")
	}

	payload, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(payload)
	if strings.Contains(encoded, "reference_solution") || strings.Contains(encoded, "explanation") {
		t.Fatalf("candidate JSON contains authoring fields: %s", encoded)
	}
}
