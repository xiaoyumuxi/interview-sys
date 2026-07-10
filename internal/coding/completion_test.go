package coding

import (
	"fmt"
	"sync"
	"testing"
)

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
	if resp.SchemaVersion != "coding.completion.v2" || resp.QuestionID != "two-sum" {
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

func TestSuggestCompletionsInfersLocalReceiverTypes(t *testing.T) {
	tests := []struct {
		name       string
		language   string
		source     string
		wantLabels []string
	}{
		{
			name:       "Java map interface",
			language:   "java",
			source:     "Map<String, Integer> counts = new HashMap<>();\ncounts.pu",
			wantLabels: []string{"put"},
		},
		{
			name:       "Java inferred constructor",
			language:   "java",
			source:     "var queue = new PriorityQueue<Integer>();\nqueue.po",
			wantLabels: []string{"poll"},
		},
		{
			name:       "Python imported alias",
			language:   "python",
			source:     "from collections import deque as D\nqueue = D()\nqueue.pop",
			wantLabels: []string{"pop", "popleft"},
		},
		{
			name:       "Python list literal",
			language:   "python",
			source:     "values = []\nvalues.ap",
			wantLabels: []string{"append"},
		},
		{
			name:       "JavaScript map constructor",
			language:   "javascript",
			source:     "const seen = new Map();\nseen.ha",
			wantLabels: []string{"has"},
		},
		{
			name:       "TypeScript array annotation",
			language:   "typescript",
			source:     "const values: Array<number> = [];\nvalues.fi",
			wantLabels: []string{"filter"},
		},
		{
			name:       "C++ vector",
			language:   "cpp",
			source:     "std::vector<int> values;\nvalues.push",
			wantLabels: []string{"push_back"},
		},
		{
			name:       "Go strings builder",
			language:   "go",
			source:     "var builder strings.Builder\nbuilder.Write",
			wantLabels: []string{"WriteString", "WriteRune"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := SuggestCompletions(CompletionRequest{
				Language:     tt.language,
				SourceCode:   tt.source,
				CursorOffset: len(tt.source),
				Limit:        20,
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !hasCompletionCapability(resp, "local_receiver_type_inference") {
				t.Fatalf("capabilities = %#v, want local_receiver_type_inference", resp.Capabilities)
			}
			for _, label := range tt.wantLabels {
				if !hasCompletionLabel(resp, label) {
					t.Fatalf("suggestions = %#v, want %q for source %q", resp.Suggestions, label, tt.source)
				}
			}
		})
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

func TestSuggestCompletionsSupportsConcurrentCatalogReads(t *testing.T) {
	const workers = 32
	errors := make(chan error, workers)
	var group sync.WaitGroup
	for index := 0; index < workers; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			language := "typescript"
			source := "const values: number[] = [];\nvalues.pu"
			want := "push"
			if index%2 == 0 {
				language = "java"
				source = "Map<String, Integer> values = new HashMap<>();\nvalues.pu"
				want = "put"
			}
			resp, err := SuggestCompletions(CompletionRequest{
				Language:     language,
				SourceCode:   source,
				CursorOffset: len(source),
				Limit:        20,
			}, nil)
			if err != nil {
				errors <- err
				return
			}
			if !hasCompletionLabel(resp, want) {
				errors <- fmt.Errorf("%s suggestions missing %q", language, want)
			}
		}(index)
	}
	group.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
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
