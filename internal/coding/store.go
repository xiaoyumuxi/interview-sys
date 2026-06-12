package coding

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

type QuestionSet struct {
	SetID        string `json:"set_id"`
	DisplayName  string `json:"display_name"`
	Description  string `json:"description"`
	Source       string `json:"source"`
	SourceURL    string `json:"source_url"`
	QuestionType string `json:"question_type"`
}

type Question struct {
	QuestionID        string   `json:"question_id"`
	SetID             string   `json:"set_id"`
	Title             string   `json:"title"`
	Difficulty        string   `json:"difficulty"`
	Source            string   `json:"source"`
	SourceURL         string   `json:"source_url"`
	QuestionType      string   `json:"question_type"`
	FrequencyRank     *int     `json:"frequency_rank,omitempty"`
	CompanyTags       []string `json:"company_tags"`
	TopicTags         []string `json:"topic_tags"`
	Prompt            string   `json:"prompt,omitempty"`
	InputFormat       string   `json:"input_format,omitempty"`
	OutputFormat      string   `json:"output_format,omitempty"`
	ConstraintsText   string   `json:"constraints_text,omitempty"`
	ReferenceSolution string   `json:"reference_solution,omitempty"`
	Explanation       string   `json:"explanation,omitempty"`
	Status            string   `json:"status"`
}

func (s *Store) ListSets(ctx context.Context) ([]QuestionSet, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT set_id, display_name, description, source, source_url, question_type
FROM code_question_sets
ORDER BY set_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []QuestionSet
	for rows.Next() {
		var item QuestionSet
		if err := rows.Scan(&item.SetID, &item.DisplayName, &item.Description, &item.Source, &item.SourceURL, &item.QuestionType); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListQuestions(ctx context.Context, questionType string, limit int) ([]Question, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `
SELECT question_id, COALESCE(set_id,''), title, difficulty, source, source_url, question_type,
       frequency_rank, array_to_string(company_tags, ','), array_to_string(topic_tags, ','), status
FROM code_questions
WHERE status='published'`
	args := []any{}
	if strings.TrimSpace(questionType) != "" {
		args = append(args, questionType)
		query += ` AND question_type=$1`
	}
	args = append(args, limit)
	query += ` ORDER BY frequency_rank NULLS LAST, title LIMIT $` + strconv.Itoa(len(args))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Question
	for rows.Next() {
		var item Question
		var rank sql.NullInt64
		var companyTags, topicTags string
		if err := rows.Scan(&item.QuestionID, &item.SetID, &item.Title, &item.Difficulty, &item.Source, &item.SourceURL, &item.QuestionType, &rank, &companyTags, &topicTags, &item.Status); err != nil {
			return nil, err
		}
		item.CompanyTags = splitTags(companyTags)
		item.TopicTags = splitTags(topicTags)
		if rank.Valid {
			v := int(rank.Int64)
			item.FrequencyRank = &v
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetQuestion(ctx context.Context, id string) (Question, bool, error) {
	var item Question
	var rank sql.NullInt64
	var companyTags, topicTags string
	err := s.db.QueryRowContext(ctx, `
SELECT question_id, COALESCE(set_id,''), title, difficulty, source, source_url, question_type,
       frequency_rank, array_to_string(company_tags, ','), array_to_string(topic_tags, ','), prompt, input_format, output_format,
       constraints_text, reference_solution, explanation, status
FROM code_questions
WHERE question_id=$1`, id).Scan(
		&item.QuestionID, &item.SetID, &item.Title, &item.Difficulty, &item.Source, &item.SourceURL, &item.QuestionType,
		&rank, &companyTags, &topicTags, &item.Prompt, &item.InputFormat, &item.OutputFormat,
		&item.ConstraintsText, &item.ReferenceSolution, &item.Explanation, &item.Status,
	)
	if err == sql.ErrNoRows {
		return Question{}, false, nil
	}
	if err != nil {
		return Question{}, false, err
	}
	if rank.Valid {
		v := int(rank.Int64)
		item.FrequencyRank = &v
	}
	item.CompanyTags = splitTags(companyTags)
	item.TopicTags = splitTags(topicTags)
	return item, true, nil
}

func splitTags(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
