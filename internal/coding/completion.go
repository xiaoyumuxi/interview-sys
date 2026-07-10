package coding

import (
	_ "embed"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
)

//go:embed completion_catalog.json
var completionCatalogJSON []byte

var (
	completionCatalogOnce sync.Once
	completionCatalogData completionCatalog
	completionCatalogErr  error
)

type completionCatalog struct {
	Languages map[string]completionCatalogLanguage `json:"languages"`
}

type completionCatalogLanguage struct {
	Extends string                            `json:"extends"`
	Globals []CompletionSuggestion            `json:"globals"`
	Members map[string][]CompletionSuggestion `json:"members"`
	Types   map[string]string                 `json:"types"`
}

type CompletionRequest struct {
	QuestionID    string `json:"question_id"`
	Language      string `json:"language"`
	SourceCode    string `json:"source_code"`
	CursorOffset  int    `json:"cursor_offset"`
	Prefix        string `json:"prefix"`
	Limit         int    `json:"limit"`
	QuestionTitle string `json:"-"`
}

type CompletionResponse struct {
	SchemaVersion string                 `json:"schema_version"`
	QuestionID    string                 `json:"question_id,omitempty"`
	Language      string                 `json:"language"`
	Prefix        string                 `json:"prefix"`
	CursorOffset  int                    `json:"cursor_offset"`
	Capabilities  []string               `json:"capabilities"`
	Suggestions   []CompletionSuggestion `json:"suggestions"`
	Diagnostics   []CompletionDiagnostic `json:"diagnostics"`
}

type CompletionSuggestion struct {
	ID         string   `json:"id"`
	Label      string   `json:"label"`
	Detail     string   `json:"detail"`
	InsertText string   `json:"insert_text"`
	Kind       string   `json:"kind"`
	Source     string   `json:"source"`
	Rank       int      `json:"rank"`
	Tags       []string `json:"tags"`
}

type CompletionDiagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type completionContext struct {
	query     string
	qualifier string
	member    bool
	cursor    int
}

const maxCompletionSourceBytes = 200000

func SuggestCompletions(req CompletionRequest, question *Question) (CompletionResponse, error) {
	language := normalizeLanguage(req.Language)
	if language == "python3" {
		language = "python"
	}
	if !supportedLanguage(language) {
		return CompletionResponse{}, errors.New("language is not supported")
	}
	if len(req.SourceCode) > maxCompletionSourceBytes {
		return CompletionResponse{}, errors.New("source_code is too large")
	}
	limit := req.Limit
	if limit <= 0 || limit > 50 {
		limit = 24
	}
	ctx := newCompletionContext(req)
	resp := CompletionResponse{
		SchemaVersion: "coding.completion.v2",
		Language:      language,
		Prefix:        ctx.query,
		CursorOffset:  ctx.cursor,
		Capabilities: []string{
			"starter_templates",
			"standard_library_members",
			"local_symbol_scan",
			"local_receiver_type_inference",
			"prefix_filtering",
			"data_driven_standard_library_catalog",
		},
		Suggestions: make([]CompletionSuggestion, 0, limit),
		Diagnostics: make([]CompletionDiagnostic, 0),
	}
	if question != nil {
		resp.QuestionID = question.QuestionID
		resp.Capabilities = append(resp.Capabilities, "question_aware_patterns")
	} else if strings.TrimSpace(req.QuestionID) != "" {
		resp.Diagnostics = append(resp.Diagnostics, CompletionDiagnostic{
			Code:    "question_context_missing",
			Message: "question context was not available while building completion suggestions",
		})
	}

	var suggestions []CompletionSuggestion
	if ctx.member {
		suggestions = append(suggestions, standardLibraryMemberSuggestions(language, req.SourceCode, ctx)...)
	} else {
		suggestions = append(suggestions, localSymbolSuggestions(language, req.SourceCode)...)
		suggestions = append(suggestions, questionPatternSuggestions(language, question)...)
		suggestions = append(suggestions, starterSuggestions(language)...)
		suggestions = append(suggestions, standardLibrarySuggestions(language)...)
	}
	resp.Suggestions = limitSuggestions(dedupeCompletionSuggestions(filterCompletionSuggestions(suggestions, ctx.query)), limit)
	return resp, nil
}

func newCompletionContext(req CompletionRequest) completionContext {
	source := req.SourceCode
	cursor := req.CursorOffset
	if cursor <= 0 || cursor > len(source) {
		cursor = len(source)
	}
	before := source[:cursor]
	query := strings.TrimSpace(req.Prefix)
	if query == "" {
		query = trailingIdentifier(before)
	}
	linePrefix := before
	if index := strings.LastIndex(linePrefix, "\n"); index >= 0 {
		linePrefix = linePrefix[index+1:]
	}
	memberRe := regexp.MustCompile(`((?:[A-Za-z_$][A-Za-z0-9_$]*(?:\.|::))*[A-Za-z_$][A-Za-z0-9_$]*)(?:\.|::)([A-Za-z_$][A-Za-z0-9_$]*)?$`)
	if match := memberRe.FindStringSubmatch(linePrefix); len(match) == 3 && match[1] != "" {
		return completionContext{
			query:     match[2],
			qualifier: match[1],
			member:    true,
			cursor:    cursor,
		}
	}
	return completionContext{query: query, cursor: cursor}
}

func trailingIdentifier(value string) string {
	re := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*$`)
	return re.FindString(value)
}

func localSymbolSuggestions(language string, source string) []CompletionSuggestion {
	items := make([]CompletionSuggestion, 0)
	addMatches := func(pattern string, kind string, detail string, rank int, call bool) {
		re := regexp.MustCompile(pattern)
		for _, match := range re.FindAllStringSubmatch(source, -1) {
			if len(match) < 2 || match[1] == "" {
				continue
			}
			label := match[1]
			insertText := label
			if call {
				insertText = label + "($0)"
			}
			items = append(items, completionSuggestion("local."+kind+"."+label, label, detail, insertText, kind, "local_symbol", rank, []string{label, kind}))
		}
	}
	switch language {
	case "go":
		addMatches(`\bfunc\s+([A-Za-z_]\w*)\s*\(`, "function", "local Go function", 20, true)
		addMatches(`\b(?:var|const)\s+([A-Za-z_]\w*)\b`, "variable", "local Go binding", 30, false)
		addMatches(`\b([A-Za-z_]\w*)\s*:=`, "variable", "local Go binding", 30, false)
		addMatches(`\btype\s+([A-Za-z_]\w*)\s+(?:struct|interface)\b`, "class", "local Go type", 35, false)
	case "java":
		addMatches(`\b(?:class|interface|enum)\s+([A-Za-z_]\w*)\b`, "class", "local Java type", 20, false)
		addMatches(`\b(?:public|private|protected)?\s*(?:static\s+)?[\w<>\[\], ?]+\s+([A-Za-z_]\w*)\s*\(`, "function", "local Java method", 25, true)
		addMatches(`\b(?:int|long|double|float|boolean|char|String|var|List<[^>]+>|Map<[^>]+>)\s+([A-Za-z_]\w*)\b`, "variable", "local Java variable", 35, false)
	case "python":
		addMatches(`\bdef\s+([A-Za-z_]\w*)\s*\(`, "function", "local Python function", 20, true)
		addMatches(`\bclass\s+([A-Za-z_]\w*)\s*[:(]`, "class", "local Python class", 25, false)
		addMatches(`(?m)^\s*([A-Za-z_]\w*)\s*=`, "variable", "local Python variable", 35, false)
	case "javascript", "typescript":
		addMatches(`\bfunction\s+([A-Za-z_$][\w$]*)\s*\(`, "function", "local function", 20, true)
		addMatches(`\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?\(?[^=;]*\)?\s*=>`, "function", "local function", 25, true)
		addMatches(`\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)\b`, "variable", "local binding", 35, false)
		addMatches(`\bclass\s+([A-Za-z_$][\w$]*)\b`, "class", "local class", 25, false)
	case "cpp", "c++":
		addMatches(`\b(?:int|long|double|float|bool|void|string|auto|vector<[^>]+>)\s+([A-Za-z_]\w*)\s*\(`, "function", "local C++ function", 20, true)
		addMatches(`\b(?:int|long|double|float|bool|string|auto|vector<[^>]+>|unordered_map<[^>]+>)\s+([A-Za-z_]\w*)\b`, "variable", "local C++ variable", 35, false)
		addMatches(`\b(?:class|struct)\s+([A-Za-z_]\w*)\b`, "class", "local C++ type", 25, false)
	}
	return items
}

func questionPatternSuggestions(language string, question *Question) []CompletionSuggestion {
	if question == nil {
		return nil
	}
	tags := normalizedTagSet(question.TopicTags)
	items := make([]CompletionSuggestion, 0)
	if tags["hash-table"] || tags["hash"] || tags["map"] {
		items = append(items, languageSnippet(language, "question.hash-map", "hash map lookup", "question-aware hash table pattern", hashMapSnippet(language), []string{"hash", "map", "two-sum", "frequency"}, 40))
	}
	if tags["array"] || tags["two-pointers"] {
		items = append(items, languageSnippet(language, "question.two-pointers", "two pointers", "left/right scan pattern", twoPointersSnippet(language), []string{"array", "two", "pointers"}, 45))
	}
	if tags["binary-search"] || tags["binary_search"] {
		items = append(items, languageSnippet(language, "question.binary-search", "binary search", "lower-bound style search", binarySearchSnippet(language), []string{"binary", "search", "lower-bound"}, 45))
	}
	if tags["dynamic-programming"] || tags["dp"] {
		items = append(items, languageSnippet(language, "question.dp", "DP table", "dynamic programming state array", dpSnippet(language), []string{"dynamic", "programming", "dp"}, 50))
	}
	if tags["string"] {
		items = append(items, languageSnippet(language, "question.string-builder", "string builder", "efficient string construction", stringBuilderSnippet(language), []string{"string", "builder"}, 50))
	}
	if tags["lru"] || tags["cache"] {
		items = append(items, languageSnippet(language, "question.cache", "cache skeleton", "backend cache/LRU interview pattern", cacheSnippet(language), []string{"cache", "lru", "backend"}, 55))
	}
	if tags["rate-limit"] || tags["rate_limiter"] || tags["limiter"] {
		items = append(items, languageSnippet(language, "question.rate-limit", "rate limiter state", "token bucket state sketch", rateLimitSnippet(language), []string{"rate", "limit", "token", "bucket"}, 55))
	}
	return items
}

func normalizedTagSet(tags []string) map[string]bool {
	out := map[string]bool{}
	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized != "" {
			out[normalized] = true
		}
	}
	return out
}

func starterSuggestions(language string) []CompletionSuggestion {
	return []CompletionSuggestion{
		completionSuggestion(language+".starter.main", "starter program", "complete runnable program template", starterSnippet(language), "snippet", "starter_template", 80, []string{"starter", "main", language}),
	}
}

func standardLibrarySuggestions(language string) []CompletionSuggestion {
	catalog, ok := completionCatalogForLanguage(language)
	if !ok {
		return nil
	}
	return catalog.Globals
}

func standardLibraryMemberSuggestions(language string, source string, ctx completionContext) []CompletionSuggestion {
	catalog, ok := completionCatalogForLanguage(language)
	if !ok {
		return nil
	}
	qualifier := ctx.qualifier
	if language == "go" {
		qualifier = resolveGoImportQualifier(source, qualifier)
	}
	items := catalog.Members[qualifier]
	if len(items) == 0 {
		if inferredType := inferReceiverType(language, sourceBeforeCursor(source, ctx.cursor), ctx.qualifier); inferredType != "" {
			items = catalog.Members[resolveCatalogType(catalog.Types, inferredType)]
		}
	}
	return filterCompletionSuggestions(items, ctx.query)
}

func sourceBeforeCursor(source string, cursor int) string {
	if cursor < 0 || cursor > len(source) {
		return source
	}
	return source[:cursor]
}

func resolveCatalogType(types map[string]string, inferredType string) string {
	typeName := strings.TrimSpace(inferredType)
	if resolved := types[typeName]; resolved != "" {
		return resolved
	}
	typeName = strings.TrimLeft(typeName, "*&")
	if strings.HasSuffix(typeName, "[]") {
		if resolved := types["Array"]; resolved != "" {
			return resolved
		}
	}
	if generic := strings.IndexAny(typeName, "<["); generic >= 0 {
		typeName = typeName[:generic]
	}
	typeName = strings.TrimPrefix(typeName, "std::")
	if dot := strings.LastIndex(typeName, "."); dot >= 0 {
		typeName = typeName[dot+1:]
	}
	if resolved := types[typeName]; resolved != "" {
		return resolved
	}
	return typeName
}

func inferReceiverType(language string, source string, receiver string) string {
	if !regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`).MatchString(receiver) {
		return ""
	}
	name := regexp.QuoteMeta(receiver)
	switch language {
	case "java":
		if value := lastCapturedMatch(source, `\b`+name+`\s*=\s*new\s+([A-Za-z_$][\w$.,]*(?:\s*<[^;(){}=]*>)?)\s*\(`); value != "" {
			return value
		}
		return lastCapturedMatch(source, `\b([A-Za-z_$][\w$.]*(?:\s*<[^;(){}=]+>)?(?:\[\])?)\s+`+name+`\s*(?:=|;|,)`)
	case "python":
		if value := lastCapturedMatch(source, `(?m)^\s*`+name+`\s*:\s*([A-Za-z_][\w.\[\], ]*)\s*(?:=|$)`); value != "" {
			return value
		}
		if value := lastCapturedMatch(source, `(?m)^\s*`+name+`\s*=\s*([A-Za-z_][\w.]*)\s*\(`); value != "" {
			return resolvePythonImportedType(source, value)
		}
		return inferLiteralType(source, name, map[string]string{
			`\[`:   "list",
			`\{`:   "dict",
			`["']`: "str",
		})
	case "javascript", "typescript":
		if value := lastCapturedMatch(source, `\b(?:const|let|var)\s+`+name+`\s*:\s*([A-Za-z_$][\w$<>,.\[\] ]*)\s*(?:=|;)`); value != "" {
			return value
		}
		if value := lastCapturedMatch(source, `\b(?:const|let|var)\s+`+name+`\s*=\s*new\s+([A-Za-z_$][\w$]*)\s*(?:<[^;(){}=]*>)?\s*\(`); value != "" {
			return value
		}
		return inferLiteralType(source, name, map[string]string{
			`\[`:     "Array",
			`\{`:     "Object",
			"[`\"']": "String",
		})
	case "cpp", "c++":
		return lastCapturedMatch(source, `\b((?:std::)?[A-Za-z_]\w*(?:\s*<[^;{}=]+>)?)\s+`+name+`\s*(?:[;={]|\(|$)`)
	case "go":
		if value := lastCapturedMatch(source, `\bvar\s+`+name+`\s+([*\[\]A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?)`); value != "" {
			return value
		}
		if value := lastCapturedMatch(source, `\b`+name+`\s*:=\s*&?([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?)\s*\{`); value != "" {
			return value
		}
		return lastCapturedMatch(source, `\b`+name+`\s*:=\s*new\(\s*([^)]+)\s*\)`)
	}
	return ""
}

func lastCapturedMatch(source string, pattern string) string {
	matches := regexp.MustCompile(pattern).FindAllStringSubmatch(source, -1)
	if len(matches) == 0 {
		return ""
	}
	match := matches[len(matches)-1]
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func inferLiteralType(source string, name string, literals map[string]string) string {
	for pattern, typeName := range literals {
		if regexp.MustCompile(`(?m)^\s*(?:const\s+|let\s+|var\s+)?` + name + `\s*=\s*` + pattern).MatchString(source) {
			return typeName
		}
	}
	return ""
}

func resolvePythonImportedType(source string, typeName string) string {
	if dot := strings.LastIndex(typeName, "."); dot >= 0 {
		return typeName[dot+1:]
	}
	for _, match := range regexp.MustCompile(`(?m)^\s*from\s+[\w.]+\s+import\s+([A-Za-z_]\w*)(?:\s+as\s+([A-Za-z_]\w*))?`).FindAllStringSubmatch(source, -1) {
		if len(match) < 3 {
			continue
		}
		alias := match[2]
		if alias == "" {
			alias = match[1]
		}
		if alias == typeName {
			return match[1]
		}
	}
	return typeName
}

func completionCatalogForLanguage(language string) (completionCatalogLanguage, bool) {
	completionCatalogOnce.Do(func() {
		completionCatalogErr = json.Unmarshal(completionCatalogJSON, &completionCatalogData)
		if completionCatalogErr != nil {
			return
		}
		for name, catalog := range completionCatalogData.Languages {
			if catalog.Extends == "" {
				continue
			}
			if base, ok := completionCatalogData.Languages[catalog.Extends]; ok {
				completionCatalogData.Languages[name] = inheritCompletionCatalog(catalog, base)
			}
		}
	})
	if completionCatalogErr != nil || completionCatalogData.Languages == nil {
		return completionCatalogLanguage{}, false
	}
	if language == "c++" {
		language = "cpp"
	}
	if language == "python3" {
		language = "python"
	}
	catalog, ok := completionCatalogData.Languages[language]
	return catalog, ok
}

func inheritCompletionCatalog(catalog completionCatalogLanguage, base completionCatalogLanguage) completionCatalogLanguage {
	types := make(map[string]string, len(base.Types)+len(catalog.Types))
	for typeName, memberGroup := range catalog.Types {
		types[typeName] = memberGroup
	}
	for typeName, memberGroup := range base.Types {
		if types[typeName] == "" {
			types[typeName] = memberGroup
		}
	}
	catalog.Types = types
	membersByQualifier := make(map[string][]CompletionSuggestion, len(base.Members)+len(catalog.Members))
	for qualifier, members := range catalog.Members {
		membersByQualifier[qualifier] = members
	}
	for qualifier, members := range base.Members {
		if len(membersByQualifier[qualifier]) == 0 {
			membersByQualifier[qualifier] = members
		}
	}
	catalog.Members = membersByQualifier
	return catalog
}

func resolveGoImportQualifier(source string, qualifier string) string {
	aliases := map[string]string{}
	for _, match := range regexp.MustCompile(`import\s+(?:(\w+)\s+)?["]([^"]+)["]`).FindAllStringSubmatch(source, -1) {
		if len(match) < 3 {
			continue
		}
		path := match[2]
		packageName := path[strings.LastIndex(path, "/")+1:]
		alias := match[1]
		if alias == "" {
			alias = packageName
		}
		aliases[alias] = packageName
	}
	if block := regexp.MustCompile(`import\s*\(([\s\S]*?)\)`).FindStringSubmatch(source); len(block) == 2 {
		for _, match := range regexp.MustCompile(`(?m)^\s*(?:(\w+)\s+)?["]([^"]+)["]`).FindAllStringSubmatch(block[1], -1) {
			if len(match) < 3 {
				continue
			}
			path := match[2]
			packageName := path[strings.LastIndex(path, "/")+1:]
			alias := match[1]
			if alias == "" {
				alias = packageName
			}
			aliases[alias] = packageName
		}
	}
	if resolved := aliases[qualifier]; resolved != "" {
		return resolved
	}
	return qualifier
}

func filterCompletionSuggestions(items []CompletionSuggestion, query string) []CompletionSuggestion {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return items
	}
	out := make([]CompletionSuggestion, 0, len(items))
	for _, item := range items {
		if suggestionMatches(item, query) {
			out = append(out, item)
		}
	}
	return out
}

func suggestionMatches(item CompletionSuggestion, query string) bool {
	values := []string{item.ID, item.Label, item.Detail, item.Kind, item.Source}
	values = append(values, item.Tags...)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func dedupeCompletionSuggestions(items []CompletionSuggestion) []CompletionSuggestion {
	seen := map[string]bool{}
	out := make([]CompletionSuggestion, 0, len(items))
	for _, item := range items {
		key := item.Source + ":" + strings.ToLower(item.Label)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func limitSuggestions(items []CompletionSuggestion, limit int) []CompletionSuggestion {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Rank == items[j].Rank {
			return items[i].Label < items[j].Label
		}
		return items[i].Rank < items[j].Rank
	})
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func languageSnippet(language string, id string, label string, detail string, insertText string, tags []string, rank int) CompletionSuggestion {
	return completionSuggestion(language+"."+id, label, detail, insertText, "snippet", "question_pattern", rank, tags)
}

func completionSuggestion(id string, label string, detail string, insertText string, kind string, source string, rank int, tags []string) CompletionSuggestion {
	return CompletionSuggestion{
		ID:         id,
		Label:      label,
		Detail:     detail,
		InsertText: insertText,
		Kind:       kind,
		Source:     source,
		Rank:       rank,
		Tags:       tags,
	}
}

func hashMapSnippet(language string) string {
	switch language {
	case "go":
		return "seen := map[int]int{}\nfor i, value := range nums {\n\tif j, ok := seen[target-value]; ok {\n\t\treturn []int{j, i}\n\t}\n\tseen[value] = i\n}"
	case "java":
		return "Map<Integer, Integer> seen = new HashMap<>();\nfor (int i = 0; i < nums.length; i++) {\n    int need = target - nums[i];\n    if (seen.containsKey(need)) return new int[]{seen.get(need), i};\n    seen.put(nums[i], i);\n}"
	case "python":
		return "seen = {}\nfor i, value in enumerate(nums):\n    need = target - value\n    if need in seen:\n        return [seen[need], i]\n    seen[value] = i"
	case "javascript":
		return "const seen = new Map();\nfor (let i = 0; i < nums.length; i++) {\n  const need = target - nums[i];\n  if (seen.has(need)) return [seen.get(need), i];\n  seen.set(nums[i], i);\n}"
	case "typescript":
		return "const seen = new Map<number, number>();\nfor (let i = 0; i < nums.length; i++) {\n  const need = target - nums[i];\n  if (seen.has(need)) return [seen.get(need)!, i];\n  seen.set(nums[i], i);\n}"
	default:
		return "unordered_map<int, int> seen;\nfor (int i = 0; i < (int)nums.size(); i++) {\n    int need = target - nums[i];\n    if (seen.count(need)) return {seen[need], i};\n    seen[nums[i]] = i;\n}"
	}
}

func twoPointersSnippet(language string) string {
	switch language {
	case "go":
		return "left, right := 0, len(nums)-1\nfor left < right {\n\t$0\n}"
	case "java":
		return "int left = 0, right = nums.length - 1;\nwhile (left < right) {\n    $0\n}"
	case "python":
		return "left, right = 0, len(nums) - 1\nwhile left < right:\n    $0"
	case "javascript", "typescript":
		return "let left = 0;\nlet right = nums.length - 1;\nwhile (left < right) {\n  $0\n}"
	default:
		return "int left = 0, right = (int)nums.size() - 1;\nwhile (left < right) {\n    $0\n}"
	}
}

func binarySearchSnippet(language string) string {
	switch language {
	case "go":
		return "left, right := 0, len(nums)\nfor left < right {\n\tmid := left + (right-left)/2\n\tif nums[mid] >= target {\n\t\tright = mid\n\t} else {\n\t\tleft = mid + 1\n\t}\n}"
	case "java":
		return "int left = 0, right = nums.length;\nwhile (left < right) {\n    int mid = left + (right - left) / 2;\n    if (nums[mid] >= target) right = mid;\n    else left = mid + 1;\n}"
	case "python":
		return "left, right = 0, len(nums)\nwhile left < right:\n    mid = left + (right - left) // 2\n    if nums[mid] >= target:\n        right = mid\n    else:\n        left = mid + 1"
	case "javascript", "typescript":
		return "let left = 0;\nlet right = nums.length;\nwhile (left < right) {\n  const mid = left + Math.floor((right - left) / 2);\n  if (nums[mid] >= target) right = mid;\n  else left = mid + 1;\n}"
	default:
		return "int left = 0, right = nums.size();\nwhile (left < right) {\n    int mid = left + (right - left) / 2;\n    if (nums[mid] >= target) right = mid;\n    else left = mid + 1;\n}"
	}
}

func dpSnippet(language string) string {
	switch language {
	case "go":
		return "dp := make([]int, n+1)\ndp[0] = 1\nfor i := 1; i <= n; i++ {\n\t$0\n}"
	case "java":
		return "int[] dp = new int[n + 1];\ndp[0] = 1;\nfor (int i = 1; i <= n; i++) {\n    $0\n}"
	case "python":
		return "dp = [0] * (n + 1)\ndp[0] = 1\nfor i in range(1, n + 1):\n    $0"
	case "javascript", "typescript":
		return "const dp = Array(n + 1).fill(0);\ndp[0] = 1;\nfor (let i = 1; i <= n; i++) {\n  $0\n}"
	default:
		return "vector<int> dp(n + 1);\ndp[0] = 1;\nfor (int i = 1; i <= n; i++) {\n    $0\n}"
	}
}

func stringBuilderSnippet(language string) string {
	switch language {
	case "go":
		return "var builder strings.Builder\nfor _, ch := range s {\n\tbuilder.WriteRune(ch)\n}\nreturn builder.String()"
	case "java":
		return "StringBuilder builder = new StringBuilder();\nfor (char ch : s.toCharArray()) {\n    builder.append(ch);\n}\nreturn builder.toString();"
	case "python":
		return "parts = []\nfor ch in s:\n    parts.append(ch)\nreturn \"\".join(parts)"
	case "javascript", "typescript":
		return "const parts = [];\nfor (const ch of s) {\n  parts.push(ch);\n}\nreturn parts.join(\"\");"
	default:
		return "string out;\nfor (char ch : s) {\n    out.push_back(ch);\n}\nreturn out;"
	}
}

func cacheSnippet(language string) string {
	switch language {
	case "go":
		return "type entry struct {\n\tkey int\n\tvalue int\n}\ncache := map[int]*list.Element{}"
	case "java":
		return "Map<Integer, Integer> cache = new LinkedHashMap<>(capacity, 0.75f, true);"
	case "python":
		return "from collections import OrderedDict\ncache = OrderedDict()"
	case "javascript", "typescript":
		return "const cache = new Map();\nconst touch = (key) => {\n  const value = cache.get(key);\n  cache.delete(key);\n  cache.set(key, value);\n};"
	default:
		return "list<pair<int, int>> order;\nunordered_map<int, list<pair<int, int>>::iterator> cache;"
	}
}

func rateLimitSnippet(language string) string {
	switch language {
	case "go":
		return "type bucket struct {\n\ttokens int\n\tupdatedAt time.Time\n}"
	case "java":
		return "class Bucket {\n    long tokens;\n    long updatedAtMillis;\n}"
	case "python":
		return "bucket = {\"tokens\": capacity, \"updated_at\": time.time()}"
	case "javascript", "typescript":
		return "const bucket = { tokens: capacity, updatedAt: Date.now() };"
	default:
		return "struct Bucket {\n    long long tokens;\n    long long updatedAtMillis;\n};"
	}
}

func starterSnippet(language string) string {
	switch language {
	case "go":
		return "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"ready\")\n}"
	case "java":
		return "import java.util.*;\n\npublic class Main {\n    public static void main(String[] args) {\n        Scanner scanner = new Scanner(System.in);\n        System.out.println(\"ready\");\n    }\n}"
	case "python":
		return "import sys\n\n\ndef main():\n    data = sys.stdin.read().strip().split()\n    print(\"ready\")\n\n\nif __name__ == \"__main__\":\n    main()"
	case "javascript":
		return "const fs = require(\"fs\");\nconst input = fs.readFileSync(0, \"utf8\").trim();\n\nfunction main() {\n  console.log(input);\n}\n\nmain();"
	case "typescript":
		return "const decoder = new TextDecoder();\nlet input = \"\";\nfor await (const chunk of Deno.stdin.readable) {\n  input += decoder.decode(chunk);\n}\n\nfunction main(): void {\n  console.log(\"ready\");\n}\n\nmain();"
	default:
		return "#include <bits/stdc++.h>\nusing namespace std;\n\nint main() {\n    ios::sync_with_stdio(false);\n    cin.tie(nullptr);\n    cout << \"ready\" << '\\n';\n    return 0;\n}"
	}
}
