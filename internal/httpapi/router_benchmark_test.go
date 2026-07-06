package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"ai-interview-platform/internal/config"
)

func BenchmarkHealthz(b *testing.B) {
	router := NewRouter(Dependencies{
		Config: config.Config{AppEnv: "benchmark"},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			b.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
		}
	}
}
