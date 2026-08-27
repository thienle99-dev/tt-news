package translation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type testRoundTripper func(*http.Request) (*http.Response, error)

func (fn testRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestModelsAndChatUsePathsBelowBaseURL(t *testing.T) {
	requests := []string{}
	httpClient := &http.Client{Transport: testRoundTripper(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		body := ""
		switch r.URL.Path {
		case "/gateway/v1/models":
			data, _ := json.Marshal(map[string]any{"data": []map[string]string{{"id": "model-z"}, {"id": "model-a", "owned_by": "test"}}})
			body = string(data)
		case "/gateway/v1/chat/completions":
			data, _ := json.Marshal(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": "OK"}}}})
			body = string(data)
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: io.NopCloser(strings.NewReader("not found")), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	client := Client{URL: "https://example.test/gateway/v1/", APIKey: "secret", Model: "model-a", HTTPClient: httpClient}
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
