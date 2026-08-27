package translation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	URL, APIKey, Model string
	HTTPClient         *http.Client
}

type Fields struct {
	Title, Description, Summary string
}

// Summarize rewrites a source article into an original title and a short 3-5 point brief.
func (c Client) Summarize(ctx context.Context, title, body string) (Fields, error) {
	if c.URL == "" || c.APIKey == "" {
		return Fields{}, errors.New("translation service is not configured")
	}
	if len(body) > 12000 {
		body = body[:12000]
	}
	instruction := `Write an original, factual news brief from the supplied source material. Do not copy phrases longer than necessary. Return 3 to 5 concise bullet points. Your entire response must be one JSON object with string keys "title" and "summary" only.`
	payload := map[string]any{"model": c.Model, "messages": []map[string]string{{"role": "system", "content": instruction}, {"role": "user", "content": fmt.Sprintf("source title: %s\n\nsource material:\n%s", title, body)}}, "temperature": 0.3, "stream": false, "response_format": map[string]string{"type": "json_object"}}
	data, err := json.Marshal(payload)
	if err != nil {
		return Fields{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, strings.NewReader(string(data)))
	if err != nil {
		return Fields{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return Fields{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Fields{}, fmt.Errorf("summary endpoint returned %s", res.Status)
	}
	rawResponse, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Fields{}, err
	}
	content, err := completionContent(rawResponse)
	if err != nil {
		return Fields{}, err
	}
	var response struct {
		Title   string          `json:"title"`
		Summary json.RawMessage `json:"summary"`
	}
	if err = json.Unmarshal([]byte(content), &response); err != nil {
		return Fields{}, fmt.Errorf("decode summary: %w", err)
	}
	result := Fields{Title: response.Title}
	if err = json.Unmarshal(response.Summary, &result.Summary); err != nil {
		var points []string
		if arrayErr := json.Unmarshal(response.Summary, &points); arrayErr != nil {
			return Fields{}, fmt.Errorf("decode summary: summary must be a string or list: %w", err)
		}
		cleaned := make([]string, 0, len(points))
		for _, point := range points {
			if point = strings.TrimSpace(point); point != "" {
				cleaned = append(cleaned, "• "+point)
			}
		}
		result.Summary = strings.Join(cleaned, "\n")
	}
	result.Title, result.Summary = strings.TrimSpace(result.Title), strings.TrimSpace(result.Summary)
	if result.Title == "" || result.Summary == "" {
		return Fields{}, errors.New("summary response is missing title or summary")
	}
	return result, nil
}

func (c Client) Vietnamese(ctx context.Context, fields Fields) (Fields, error) {
	if c.URL == "" || c.APIKey == "" {
		return Fields{}, errors.New("translation service is not configured")
	}
	instruction := `Translate the supplied news fields into Vietnamese faithfully. Preserve names, numbers, and factual meaning. Do not summarize or add facts.

Your entire response MUST be one valid JSON object and nothing else. The first character must be { and the last character must be }. Do not use Markdown, code fences, prose, labels, or explanations. Use exactly these string keys: "title", "description", "summary". Keep empty input fields as empty strings.

Required output shape:
{"title":"Vietnamese translation","description":"Vietnamese translation","summary":"Vietnamese translation"}`
	payload := map[string]any{"model": c.Model, "messages": []map[string]string{{"role": "system", "content": instruction}, {"role": "user", "content": fmt.Sprintf("title: %s\n\ndescription: %s\n\nsummary: %s", fields.Title, fields.Description, fields.Summary)}}, "temperature": 0.2, "stream": false, "response_format": map[string]string{"type": "json_object"}}
	data, err := json.Marshal(payload)
	if err != nil {
		return Fields{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, strings.NewReader(string(data)))
	if err != nil {
		return Fields{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return Fields{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 8<<10))
		if readErr != nil {
			return Fields{}, fmt.Errorf("translation endpoint returned %s (could not read error body: %w)", res.Status, readErr)
		}
		message := strings.TrimSpace(string(body))
		if message == "" {
			return Fields{}, fmt.Errorf("translation endpoint returned %s with an empty error body", res.Status)
		}
		return Fields{}, fmt.Errorf("translation endpoint returned %s: %s", res.Status, message)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Fields{}, fmt.Errorf("could not read translation response: %w", err)
	}
	content, err := completionContent(body)
	if err != nil {
		return Fields{}, err
	}
	var result Fields
	if err = json.Unmarshal([]byte(content), &result); err != nil {
		content := strings.TrimSpace(content)
		if len(content) > 4096 {
			content = content[:4096] + "…"
		}
		return Fields{}, fmt.Errorf("decode translation: %w; model content: %q", err, content)
	}
	result.Title, result.Description, result.Summary = strings.TrimSpace(result.Title), strings.TrimSpace(result.Description), strings.TrimSpace(result.Summary)
	if result.Title == "" {
		return Fields{}, errors.New("translation response is missing title")
	}
	return result, nil
}

func completionContent(body []byte) (string, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &response); err == nil {
		if len(response.Choices) == 0 || response.Choices[0].Message.Content == "" {
			return "", errors.New("translation response has no choices")
		}
		return response.Choices[0].Message.Content, nil
	}

	// Some OpenAI-compatible proxies ignore stream:false and send Server-Sent Events.
	// Combine each delta so those proxies still work without treating `data:` as JSON.
	var content strings.Builder
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", fmt.Errorf("decode streaming translation response: %w", err)
		}
		for _, choice := range chunk.Choices {
			content.WriteString(choice.Delta.Content)
		}
	}
	if content.Len() == 0 {
		preview := strings.TrimSpace(string(body))
		if len(preview) > 300 {
			preview = preview[:300] + "…"
		}
		return "", fmt.Errorf("translation endpoint returned an unexpected response: %q", preview)
	}
	return content.String(), nil
}
