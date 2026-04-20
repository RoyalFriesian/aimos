package aiclients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaClient_Generate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req ollamaChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.Model != "qwen2.5-coder:7b" {
			t.Errorf("unexpected model: %s", req.Model)
		}
		if req.Stream {
			t.Error("expected stream=false")
		}
		if len(req.Messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(req.Messages))
		}

		resp := ollamaChatResponse{
			Message: ollamaChatMessage{
				Role:    "assistant",
				Content: "Hello from Ollama!",
			},
			Done: true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{BaseURL: server.URL}, nil)
	result, err := client.Generate(context.Background(), "qwen2.5-coder:7b", "You are helpful.", "Say hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello from Ollama!" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestOllamaClient_Ping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"models":[]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{BaseURL: server.URL}, nil)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("expected ping success, got: %v", err)
	}
}

func TestOllamaClient_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"model not found"}`))
	}))
	defer server.Close()

	client := NewOllamaClient(OllamaConfig{BaseURL: server.URL}, nil)
	_, err := client.Generate(context.Background(), "nonexistent", "sys", "usr")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestRouterClient_OllamaPrefix(t *testing.T) {
	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "qwen2.5-coder:7b" {
			t.Errorf("expected stripped model name, got: %s", req.Model)
		}
		resp := ollamaChatResponse{
			Message: ollamaChatMessage{Role: "assistant", Content: "Ollama response"},
			Done:    true,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ollamaServer.Close()

	router := NewRouterClient(RouterConfig{
		Ollama: &OllamaConfig{BaseURL: ollamaServer.URL},
	}, nil)

	result, err := router.Generate(context.Background(), "ollama/qwen2.5-coder:7b", "sys", "usr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Ollama response" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestRouterClient_NoPrefix_FallsToOpenAI(t *testing.T) {
	router := NewRouterClient(RouterConfig{}, nil)
	_, err := router.Generate(context.Background(), "gpt-5.4", "sys", "usr")
	if err == nil {
		t.Fatal("expected error when no providers configured")
	}
}
