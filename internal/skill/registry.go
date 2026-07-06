package skill

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Skill struct {
	ID            string      `json:"id"`
	DisplayName   string      `json:"display_name"`
	Description   string      `json:"description"`
	Categories    []Category  `json:"categories"`
	Instructions  string      `json:"instructions,omitempty"`
	References    []Reference `json:"references,omitempty"`
	Lint          LintResult  `json:"lint"`
	LoadedAt      string      `json:"loaded_at"`
	SchemaVersion string      `json:"schema_version"`
}

type Category struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Priority string `json:"priority"`
	Ref      string `json:"ref,omitempty"`
}

type Reference struct {
	SourceID string `json:"source_id"`
	Content  string `json:"content"`
	Tokens   int    `json:"tokens"`
}

type LintResult struct {
	OK       bool     `json:"ok"`
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

type CreateRequest struct {
	ID           string            `json:"id"`
	DisplayName  string            `json:"display_name"`
	Description  string            `json:"description"`
	Instructions string            `json:"instructions"`
	Categories   []Category        `json:"categories"`
	References   map[string]string `json:"references"`
}

type Registry struct {
	dir    string
	mu     sync.RWMutex
	skills map[string]Skill
}

func NewRegistry(dir string) *Registry {
	return &Registry{dir: dir, skills: map[string]Skill{}}
}

func (r *Registry) Load() error {
	loaded, err := r.scan()
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills = loaded
	return nil
}

func (r *Registry) Reload() error {
	return r.Load()
}

func (r *Registry) Create(req CreateRequest) (Skill, error) {
	if err := validateCreateRequest(req); err != nil {
		return Skill{}, err
	}

	skillDir := filepath.Join(r.dir, req.ID)
	if _, err := os.Stat(skillDir); err == nil {
		return Skill{}, fmt.Errorf("skill %q already exists", req.ID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Skill{}, err
	}

	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o755); err != nil {
		return Skill{}, err
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.meta.yml"), []byte(renderMeta(req)), 0o644); err != nil {
		return Skill{}, err
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(req.Instructions), 0o644); err != nil {
		return Skill{}, err
	}
	for name, content := range req.References {
		cleanName, err := cleanReferenceName(name)
		if err != nil {
			return Skill{}, err
		}
		if err := os.WriteFile(filepath.Join(skillDir, "references", cleanName), []byte(content), 0o644); err != nil {
			return Skill{}, err
		}
	}

	if err := r.Reload(); err != nil {
		return Skill{}, err
	}
	item, ok := r.Get(req.ID)
	if !ok {
		return Skill{}, errors.New("skill created but not loaded")
	}
	return item, nil
}

func (r *Registry) scan() (map[string]Skill, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]Skill{}, nil
		}
		return nil, err
	}

	loaded := map[string]Skill{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(r.dir, entry.Name())
		metaPath := filepath.Join(skillDir, "skill.meta.yml")
		instructionPath := filepath.Join(skillDir, "SKILL.md")
		meta, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		instructions, _ := os.ReadFile(instructionPath)
		item := parseMeta(string(meta))
		if item.ID == "" {
			item.ID = entry.Name()
		}
		item.Instructions = string(instructions)
		item.References = loadReferences(skillDir, item)
		item.Lint = lintSkill(skillDir, item)
		item.LoadedAt = time.Now().Format(time.RFC3339)
		item.SchemaVersion = "skill.v1"
		loaded[item.ID] = item
	}
	return loaded, nil
}

func (r *Registry) List() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]Skill, 0, len(r.skills))
	for _, item := range r.skills {
		item.Instructions = ""
		item.References = nil
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func (r *Registry) All() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]Skill, 0, len(r.skills))
	for _, item := range r.skills {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func (r *Registry) Get(id string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.skills[id]
	return item, ok
}

var skillIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

func validateCreateRequest(req CreateRequest) error {
	if !skillIDPattern.MatchString(req.ID) {
		return errors.New("id must be kebab-case, 3-64 chars, using lowercase letters, numbers, and hyphen")
	}
	if strings.TrimSpace(req.DisplayName) == "" {
		return errors.New("display_name is required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return errors.New("description is required")
	}
	if strings.TrimSpace(req.Instructions) == "" {
		return errors.New("instructions is required")
	}
	for _, category := range req.Categories {
		if strings.TrimSpace(category.Key) == "" || strings.TrimSpace(category.Label) == "" {
			return errors.New("category key and label are required")
		}
		if category.Ref != "" {
			if _, err := cleanReferenceName(category.Ref); err != nil {
				return err
			}
		}
	}
	for name := range req.References {
		if _, err := cleanReferenceName(name); err != nil {
			return err
		}
	}
	item := Skill{
		ID:           req.ID,
		DisplayName:  req.DisplayName,
		Description:  req.Description,
		Categories:   req.Categories,
		Instructions: req.Instructions,
	}
	for name, content := range req.References {
		item.References = append(item.References, Reference{
			SourceID: req.ID + "/" + name,
			Content:  content,
			Tokens:   estimateTokens(content),
		})
	}
	lint := lintSkill("", item)
	if len(lint.Errors) > 0 {
		return fmt.Errorf("skill lint failed: %s", strings.Join(lint.Errors, "; "))
	}
	return nil
}

func cleanReferenceName(name string) (string, error) {
	raw := strings.TrimSpace(name)
	if raw == "" || strings.Contains(raw, "/") || strings.Contains(raw, "\\") {
		return "", fmt.Errorf("invalid reference name %q", name)
	}
	clean := filepath.Base(raw)
	if clean == "." || clean == "" || !strings.HasSuffix(clean, ".md") {
		return "", fmt.Errorf("invalid reference name %q", name)
	}
	return clean, nil
}

func renderMeta(req CreateRequest) string {
	var builder strings.Builder
	builder.WriteString("schemaVersion: skill.meta.v1\n")
	builder.WriteString("id: ")
	builder.WriteString(req.ID)
	builder.WriteString("\n")
	builder.WriteString("displayName: ")
	builder.WriteString(req.DisplayName)
	builder.WriteString("\n")
	builder.WriteString("description: ")
	builder.WriteString(req.Description)
	builder.WriteString("\n")
	builder.WriteString("categories:\n")
	for _, category := range req.Categories {
		builder.WriteString("  - key: ")
		builder.WriteString(category.Key)
		builder.WriteString("\n    label: ")
		builder.WriteString(category.Label)
		builder.WriteString("\n    priority: ")
		if strings.TrimSpace(category.Priority) == "" {
			builder.WriteString("NORMAL")
		} else {
			builder.WriteString(category.Priority)
		}
		if category.Ref != "" {
			builder.WriteString("\n    ref: ")
			builder.WriteString(category.Ref)
		}
		builder.WriteString("\n")
	}
	return builder.String()
}

func lintSkill(skillDir string, item Skill) LintResult {
	var warnings []string
	var errorMessages []string

	if !skillIDPattern.MatchString(item.ID) {
		errorMessages = append(errorMessages, "skill id must be kebab-case")
	}
	if len(item.Description) > 240 {
		warnings = append(warnings, "description is long; keep it concise for retrieval")
	}
	if strings.TrimSpace(item.Instructions) == "" {
		errorMessages = append(errorMessages, "SKILL.md instructions are required")
	}
	if len(item.Categories) == 0 {
		warnings = append(warnings, "skill has no categories")
	}
	if !containsAnyFold(item.Instructions, []string{"禁止", "do not", "must not", "不要"}) {
		warnings = append(warnings, "SKILL.md should include explicit forbidden behavior")
	}

	contentByRef := map[string]string{}
	for _, ref := range item.References {
		contentByRef[strings.TrimPrefix(ref.SourceID, item.ID+"/")] = ref.Content
	}
	for _, category := range item.Categories {
		if category.Ref == "" {
			continue
		}
		if _, err := cleanReferenceName(category.Ref); err != nil {
			errorMessages = append(errorMessages, err.Error())
			continue
		}
		if skillDir != "" {
			if _, err := os.Stat(filepath.Join(skillDir, "references", category.Ref)); err != nil {
				warnings = append(warnings, "category ref not found: "+category.Ref)
			}
		} else if _, ok := contentByRef[category.Ref]; !ok {
			warnings = append(warnings, "category ref not provided: "+category.Ref)
		}
	}

	scanText := item.DisplayName + "\n" + item.Description + "\n" + item.Instructions
	for _, ref := range item.References {
		scanText += "\n" + ref.Content
	}
	for _, finding := range scanPromptInjection(scanText) {
		if finding.Severity == "error" {
			errorMessages = append(errorMessages, finding.Message)
		} else {
			warnings = append(warnings, finding.Message)
		}
	}

	return LintResult{
		OK:       len(errorMessages) == 0,
		Warnings: uniqueStrings(warnings),
		Errors:   uniqueStrings(errorMessages),
	}
}

type promptInjectionFinding struct {
	Severity string
	Message  string
}

func scanPromptInjection(content string) []promptInjectionFinding {
	lower := strings.ToLower(content)
	patterns := []struct {
		needle   string
		severity string
		message  string
	}{
		{"ignore previous instructions", "error", "possible prompt injection: ignore previous instructions"},
		{"ignore all previous instructions", "error", "possible prompt injection: ignore all previous instructions"},
		{"disregard previous instructions", "error", "possible prompt injection: disregard previous instructions"},
		{"reveal your system prompt", "error", "possible prompt injection: reveal system prompt"},
		{"print your system prompt", "error", "possible prompt injection: print system prompt"},
		{"developer message", "warning", "mentions developer message; review for prompt boundary confusion"},
		{"system prompt", "warning", "mentions system prompt; review for prompt leakage request"},
		{"泄露系统提示", "error", "possible prompt injection: leak system prompt"},
		{"忽略之前", "error", "possible prompt injection: ignore previous instructions"},
		{"忽略以上", "error", "possible prompt injection: ignore above instructions"},
		{"无视之前", "error", "possible prompt injection: disregard previous instructions"},
		{"输出系统提示", "error", "possible prompt injection: print system prompt"},
		{"显示系统提示", "error", "possible prompt injection: show system prompt"},
		{"开发者消息", "warning", "mentions developer message; review for prompt boundary confusion"},
		{"系统提示词", "warning", "mentions system prompt; review for prompt leakage request"},
	}
	var findings []promptInjectionFinding
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern.needle) {
			findings = append(findings, promptInjectionFinding{
				Severity: pattern.severity,
				Message:  pattern.message,
			})
		}
	}
	return findings
}

func containsAnyFold(content string, needles []string) bool {
	lower := strings.ToLower(content)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func parseMeta(meta string) Skill {
	var item Skill
	var current *Category
	for _, rawLine := range strings.Split(meta, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "id:"):
			item.ID = cleanYAMLValue(strings.TrimPrefix(line, "id:"))
		case strings.HasPrefix(line, "displayName:"):
			item.DisplayName = cleanYAMLValue(strings.TrimPrefix(line, "displayName:"))
		case strings.HasPrefix(line, "description:"):
			item.Description = cleanYAMLValue(strings.TrimPrefix(line, "description:"))
		case strings.HasPrefix(line, "- key:"):
			category := Category{Key: cleanYAMLValue(strings.TrimPrefix(line, "- key:"))}
			item.Categories = append(item.Categories, category)
			current = &item.Categories[len(item.Categories)-1]
		case current != nil && strings.HasPrefix(line, "label:"):
			current.Label = cleanYAMLValue(strings.TrimPrefix(line, "label:"))
		case current != nil && strings.HasPrefix(line, "priority:"):
			current.Priority = cleanYAMLValue(strings.TrimPrefix(line, "priority:"))
		case current != nil && strings.HasPrefix(line, "ref:"):
			current.Ref = cleanYAMLValue(strings.TrimPrefix(line, "ref:"))
		}
	}
	return item
}

func cleanYAMLValue(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"'`)
}

func loadReferences(skillDir string, item Skill) []Reference {
	seen := map[string]bool{}
	var refs []Reference
	for _, category := range item.Categories {
		if category.Ref == "" || seen[category.Ref] {
			continue
		}
		seen[category.Ref] = true
		content, err := os.ReadFile(filepath.Join(skillDir, "references", category.Ref))
		if err != nil {
			continue
		}
		refs = append(refs, Reference{
			SourceID: item.ID + "/" + category.Ref,
			Content:  string(content),
			Tokens:   estimateTokens(string(content)),
		})
	}
	return refs
}

func estimateTokens(content string) int {
	words := len(strings.Fields(content))
	if words == 0 && content != "" {
		return len([]rune(content)) / 2
	}
	return words*2 + len([]rune(content))/8
}
