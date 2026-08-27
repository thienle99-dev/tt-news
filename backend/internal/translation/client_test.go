package translation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModelsAndChatUsePathsBelowBaseURL(t *testing.T) {
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/gateway/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "model-z"}, {"id": "model-a", "owned_by": "test"}}})
		case "/gateway/v1/chat/completions":
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": "OK"}}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := Client{URL: server.URL + "/gateway/v1/", APIKey: "secret", Model: "model-a"}
	models, err := client.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].ID != "model-a" || models[1].ID != "model-z" {
		t.Fatalf("unexpected models: %#v", models)
	}
	reply, err := client.Test(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if reply != "OK" {
		t.Fatalf("reply = %q", reply)
	}
	if len(requests) != 2 || requests[0] != "GET /gateway/v1/models" || requests[1] != "POST /gateway/v1/chat/completions" {
		t.Fatalf("unexpected requests: %#v", requests)
	}
}

func TestNormalizeBaseURLAcceptsLegacyChatEndpoint(t *testing.T) {
	got, err := NormalizeBaseURL("https://example.test/v1/chat/completions/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.test/v1" {
		t.Fatalf("base URL = %q", got)
	}
}
