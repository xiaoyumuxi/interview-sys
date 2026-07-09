package coding

import "testing"

func TestSuggestCompletionsUsesQuestionTagsAndLocalSymbols(t *testing.T) {
	resp, err := SuggestCompletions(CompletionRequest{
		Language:     "go",
		SourceCode:   "package main\n\nfunc twoSum(nums []int, target int) []int {\n\treturn nil\n}\n",
		CursorOffset: 0,
		Limit:        20,
	}, &Question{
		QuestionID: "two-sum",
		TopicTags:  []string{"array", "hash-table"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.SchemaVersion != "coding.completion.v1" || resp.QuestionID != "two-sum" {
		t.Fatalf("unexpected response identity: %+v", resp)
	}
	if !hasCompletionCapability(resp, "question_aware_patterns") {
		t.Fatalf("capabilities = %#v, want question_aware_patterns", resp.Capabilities)
	}
	if !hasCompletionLabel(resp, "twoSum") {
		t.Fatalf("suggestions = %#v, want local twoSum symbol", resp.Suggestions)
	}
	if !hasCompletionLabel(resp, "hash map lookup") {
		t.Fatalf("suggestions = %#v, want hash map question pattern", resp.Suggestions)
	}
}

func TestSuggestCompletionsFiltersMemberAccess(t *testing.T) {
	source := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Pr\n}\n"
	resp, err := SuggestCompletions(CompletionRequest{
		Language:     "go",
		SourceCode:   source,
		CursorOffset: len("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Pr"),
		Limit:        10,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Prefix != "Pr" {
		t.Fatalf("prefix = %q, want Pr", resp.Prefix)
	}
	if !hasCompletionLabel(resp, "Println") || !hasCompletionLabel(resp, "Printf") {
		t.Fatalf("suggestions = %#v, want fmt print member suggestions", resp.Suggestions)
	}
	if hasCompletionLabel(resp, "starter program") {
		t.Fatalf("member access should not include starter suggestions: %#v", resp.Suggestions)
	}
}

func TestSuggestCompletionsSupportsJavaSystemChain(t *testing.T) {
	source := "public class Main {\n    public static void main(String[] args) {\n        System.o\n    }\n}\n"
	resp, err := SuggestCompletions(CompletionRequest{
		Language:     "java",
		SourceCode:   source,
		CursorOffset: len("public class Main {\n    public static void main(String[] args) {\n        System.o"),
		Limit:        10,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Prefix != "o" {
		t.Fatalf("prefix = %q, want o", resp.Prefix)
	}
	if !hasCompletionLabel(resp, "out") {
		t.Fatalf("suggestions = %#v, want System.out property suggestion", resp.Suggestions)
	}

	source = "public class Main {\n    public static void main(String[] args) {\n        System.out.pr\n    }\n}\n"
	resp, err = SuggestCompletions(CompletionRequest{
		Language:     "java",
		SourceCode:   source,
		CursorOffset: len("public class Main {\n    public static void main(String[] args) {\n        System.out.pr"),
		Limit:        10,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCompletionLabel(resp, "println") || !hasCompletionLabel(resp, "printf") {
		t.Fatalf("suggestions = %#v, want System.out print method suggestions", resp.Suggestions)
	}
}

func TestSuggestCompletionsUsesCatalogForJavaCollections(t *testing.T) {
	resp, err := SuggestCompletions(CompletionRequest{
		Language:   "java",
		SourceCode: "import java.util.*;\nclass Main {}\n",
		Prefix:     "Hash",
		Limit:      20,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCompletionCapability(resp, "data_driven_standard_library_catalog") {
		t.Fatalf("capabilities = %#v, want catalog capability", resp.Capabilities)
	}
	if !hasCompletionLabel(resp, "HashMap") || !hasCompletionLabel(resp, "HashSet") {
		t.Fatalf("suggestions = %#v, want HashMap and HashSet catalog suggestions", resp.Suggestions)
	}

	resp, err = SuggestCompletions(CompletionRequest{
		Language:   "java",
		SourceCode: "import java.util.*;\nclass Main {}\n",
		Prefix:     "Priority",
		Limit:      20,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCompletionLabel(resp, "PriorityQueue") {
		t.Fatalf("suggestions = %#v, want PriorityQueue catalog suggestion", resp.Suggestions)
	}
}

func TestSuggestCompletionsRejectsUnsupportedLanguage(t *testing.T) {
	_, err := SuggestCompletions(CompletionRequest{
		Language:   "ruby",
		SourceCode: "puts 'nope'",
	}, nil)
	if err == nil {
		t.Fatal("SuggestCompletions() error = nil, want unsupported language error")
	}
}

func hasCompletionCapability(resp CompletionResponse, capability string) bool {
	for _, item := range resp.Capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

func hasCompletionLabel(resp CompletionResponse, label string) bool {
	for _, item := range resp.Suggestions {
		if item.Label == label {
			return true
		}
	}
	return false
}
