package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"ai-interview-platform/internal/config"
	"ai-interview-platform/internal/contextengine"
	"ai-interview-platform/internal/provider"
	"ai-interview-platform/internal/skill"
)

type Dependencies struct {
	Config           config.Config
	Logger           *slog.Logger
	ProviderRegistry *provider.Registry
	SkillRegistry    *skill.Registry
	ContextEngine    *contextengine.Engine
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	api := apiHandler{deps: deps}

	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /api/providers", api.listProviders)
	mux.HandleFunc("POST /api/providers/test", api.testProvider)
	mux.HandleFunc("GET /api/skills", api.listSkills)
	mux.HandleFunc("GET /api/skills/{skill_id}", api.getSkill)
	mux.HandleFunc("POST /api/context/preview", api.contextPreview)

	return requestLogger(deps.Logger, recoverer(mux))
}

type apiHandler struct {
	deps Dependencies
}

func (h apiHandler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"schema_version": "health.v1",
		"app_env":        h.deps.Config.AppEnv,
		"time":           time.Now().Format(time.RFC3339),
	})
}

func (h apiHandler) listProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"schema_version": "provider.list.v1",
		"items":          h.deps.ProviderRegistry.List(),
	})
}

func (h apiHandler) testProvider(w http.ResponseWriter, r *http.Request) {
	var req provider.TestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	result := h.deps.ProviderRegistry.Test(r.Context(), req)
	writeJSON(w, http.StatusOK, result)
}

func (h apiHandler) listSkills(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"schema_version": "skill.list.v1",
		"items":          h.deps.SkillRegistry.List(),
	})
}

func (h apiHandler) getSkill(w http.ResponseWriter, r *http.Request) {
	item, ok := h.deps.SkillRegistry.Get(r.PathValue("skill_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "skill_not_found", "skill_id is not registered")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h apiHandler) contextPreview(w http.ResponseWriter, r *http.Request) {
	var req contextengine.PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	resp, err := h.deps.ContextEngine.Preview(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "context_preview_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
