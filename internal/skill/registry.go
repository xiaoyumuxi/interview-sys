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
