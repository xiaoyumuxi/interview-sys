package provider

import "testing"

func TestChatEndpoint(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "deepseek openai format",
			cfg: Config{
				BaseURL:          "https://api.deepseek.com/",
				ChatEndpointPath: "/chat/completions",
			},
			want: "https://api.deepseek.com/chat/completions",
		},
		{
			name: "openai compatible v1 base",
			cfg: Config{
				BaseURL:          "https://api.openai.com/v1",
				ChatEndpointPath: "/chat/completions",
			},
			want: "https://api.openai.com/v1/chat/completions",
		},
		{
			name: "path without leading slash",
			cfg: Config{
				BaseURL:          "http://localhost:11434/v1/",
				ChatEndpointPath: "chat/completions",
			},
			want: "http://localhost:11434/v1/chat/completions",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ChatEndpoint(test.cfg); got != test.want {
				t.Fatalf("ChatEndpoint() = %q, want %q", got, test.want)
			}
		})
	}
}
