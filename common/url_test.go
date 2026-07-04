package common

import "testing"

func TestJoinURLPath(t *testing.T) {
	tests := []struct {
		name string
		base string
		path string
		want string
	}{
		{
			name: "base without trailing slash",
			base: "https://api.example.com/v1",
			path: "chat/completions",
			want: "https://api.example.com/v1/chat/completions",
		},
		{
			name: "base with trailing slash",
			base: "https://api.example.com/v1/",
			path: "/chat/completions",
			want: "https://api.example.com/v1/chat/completions",
		},
		{
			name: "base without path",
			base: "https://api.example.com",
			path: "chat/completions",
			want: "https://api.example.com/chat/completions",
		},
		{
			name: "preserves query",
			base: "https://proxy.example.com/root/?token=a",
			path: "messages",
			want: "https://proxy.example.com/root/messages?token=a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JoinURLPath(tt.base, tt.path); got != tt.want {
				t.Fatalf("JoinURLPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJoinURLPathWithVersion(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		version string
		path    string
		want    string
	}{
		{
			name:    "adds missing version after provider entrypoint",
			base:    "https://api.longcat.chat/anthropic",
			version: "v1",
			path:    "messages",
			want:    "https://api.longcat.chat/anthropic/v1/messages",
		},
		{
			name:    "does not duplicate existing version",
			base:    "https://api.longcat.chat/anthropic/v1/",
			version: "v1",
			path:    "/messages",
			want:    "https://api.longcat.chat/anthropic/v1/messages",
		},
		{
			name:    "adds version after host-only base",
			base:    "https://api.example.com",
			version: "v1",
			path:    "chat/completions",
			want:    "https://api.example.com/v1/chat/completions",
		},
		{
			name:    "preserves query",
			base:    "https://proxy.example.com/openai?token=a",
			version: "v1",
			path:    "chat/completions",
			want:    "https://proxy.example.com/openai/v1/chat/completions?token=a",
		},
		{
			name:    "does not duplicate full endpoint",
			base:    "https://api.example.com/anthropic/v1/messages",
			version: "v1",
			path:    "messages",
			want:    "https://api.example.com/anthropic/v1/messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JoinURLPathWithVersion(tt.base, tt.version, tt.path); got != tt.want {
				t.Fatalf("JoinURLPathWithVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
