package skill

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

type Registry struct {
	dir    string
	skills map[string]Skill
}

func NewRegistry(dir string) *Registry {
	return &Registry{dir: dir, skills: map[string]Skill{}}
}

func (r *Registry) Load() error {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
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
	r.skills = loaded
	return nil
}

func (r *Registry) List() []Skill {
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
	item, ok := r.skills[id]
	return item, ok
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
