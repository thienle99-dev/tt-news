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

type FeaturedCandidate struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	Source      string `json:"source"`
	Category    string `json:"category"`
	PublishedAt string `json:"published_at"`
}

type FeaturedTopic struct {
	Title      string  `json:"title"`
	Summary    string  `json:"summary"`
	ArticleIDs []int64 `json:"article_ids"`
}

type FeaturedBrief struct {
	Title  string          `json:"title"`
	Intro  string          `json:"intro"`
	Topics []FeaturedTopic `json:"topics"`
}

func (c Client) Featured(ctx context.Context, candidates []FeaturedCandidate) (FeaturedBrief, error) {
	instruction := `You are the editor of an international news briefing. Group the supplied articles into 5 to 8 distinct, important news events. Return only JSON with title, intro, and topics. Each topic needs title, summary, and article_ids. Summary must be concise and factual. Each topic must cite 1 to 3 supplied article IDs, never invent IDs, and no ID may appear in more than one topic. Prefer diverse sources.`
	return c.featuredRequest(ctx, instruction, candidates)
}

func (c Client) FeaturedVietnamese(ctx context.Context, brief FeaturedBrief) (FeaturedBrief, error) {
	instruction := `Translate this featured news briefing into natural Vietnamese. Preserve article_ids exactly. Return only JSON with title, intro, and topics; every topic must have title, summary, and article_ids. Do not add, remove, or reorder topics or IDs.`
	return c.featuredRequest(ctx, instruction, brief)
}

func (c Client) featuredRequest(ctx context.Context, instruction string, input any) (FeaturedBrief, error) {
	if c.URL == "" || c.APIKey == "" {
		return FeaturedBrief{}, errors.New("translation service is not configured")
	}
	payload := map[string]any{"model": c.Model, "messages": []map[string]string{{"role": "system", "content": instruction}, {"role": "user", "content": fmt.Sprintf("data:\n%s", mustJSON(input))}}, "temperature": 0.2, "stream": false, "response_format": map[string]string{"type": "json_object"}}
	data, err := json.Marshal(payload)
	if err != nil {
		return FeaturedBrief{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, strings.NewReader(string(data)))
	if err != nil {
		return FeaturedBrief{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return FeaturedBrief{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return FeaturedBrief{}, fmt.Errorf("featured endpoint returned %s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return FeaturedBrief{}, err
	}
	content, err := completionContent(body)
	if err != nil {
		return FeaturedBrief{}, err
	}
	var brief FeaturedBrief
	if err = json.Unmarshal([]byte(content), &brief); err != nil {
		return FeaturedBrief{}, fmt.Errorf("decode featured briefing: %w", err)
	}
	brief.Title, brief.Intro = strings.TrimSpace(brief.Title), strings.TrimSpace(brief.Intro)
	return brief, nil
}

func mustJSON(value any) string { data, _ := json.Marshal(value); return string(data) }

func (c Client) IsJunk(ctx context.Context, title string) (bool, error) {
	if c.URL == "" || c.APIKey == "" {
		return false, errors.New("translation service is not configured")
	}
	instruction := `Classify whether an RSS item is unsuitable for an international news feed using only its title. Mark junk=true for weather forecasts or routine weather updates, advertisements or sponsored PR, podcasts/videos/audio-only posts, photo galleries, entertainment, celebrity, lifestyle, fashion, food, travel, or extremely thin or malformed items. Do not mark ordinary reporting, analysis, opinion, public-safety weather emergencies, culture with public significance, or sports news as junk. When the title alone is insufficient, return junk=false. Return only JSON: {"junk":true|false}.`
	payload := map[string]any{"model": c.Model, "messages": []map[string]string{{"role": "system", "content": instruction}, {"role": "user", "content": fmt.Sprintf("title: %s", title)}}, "temperature": 0, "stream": false, "response_format": map[string]string{"type": "json_object"}}
	data, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, strings.NewReader(string(data)))
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return false, fmt.Errorf("content review endpoint returned %s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return false, err
	}
	content, err := completionContent(body)
	if err != nil {
		return false, err
	}
	var result struct {
		Junk bool `json:"junk"`
	}
	if err = json.Unmarshal([]byte(content), &result); err != nil {
		return false, fmt.Errorf("decode content review: %w", err)
	}
	return result.Junk, nil
}

// Summarize rewrites a source article into an original title and a detailed,
// content-proportional brief.
func (c Client) Summarize(ctx context.Context, title, body string) (Fields, error) {
	if c.URL == "" || c.APIKey == "" {
		return Fields{}, errors.New("translation service is not configured")
	}
	if len(body) > 12000 {
		body = body[:12000]
	}
	instruction := `You are a meticulous news editor. Create an original, self-contained, factual brief from the supplied full article, not merely from its headline.

Work silently in two passes. First build a fact inventory from the source: the central event or announcement; the people, organizations, and products involved; date and location; price and availability; specifications, measurements, materials, performance figures, and test conditions; notable features; practical benefits or consequences; comparisons, limitations, and what happens next. Then select and organize the facts that let a reader understand the article without opening the source.

The amount of detail must follow the source. For a substantive article, write 7 to 10 bullets and approximately 160 to 240 words in total. A product article containing at least five distinct supported facts or specification groups is substantive and MUST use this 7-to-10-bullet range; do not classify it as sparse merely because the central announcement is simple. Only when the entire source genuinely supplies fewer than five distinct facts may you use 4 to 6 bullets and fewer words rather than padding or inventing information. Each bullet should normally contain 1 to 2 complete sentences and combine closely related facts into a useful point; do not reduce a bullet to a label or a single isolated number. Begin with a clear overview of what happened, then move through the most relevant supporting detail in a logical order.

Adapt coverage to the subject. For a product launch, include, when supported: what the product is and where it is launching; campaign or sale dates; local price and any converted price; capacity or dimensions; construction and materials; measured or claimed performance with temperatures or duration; opening, locking, sealing, cleaning, safety, and portability features; colors or variants; and any stated availability limits. For other news, prioritize who did what, when and where, the evidence or key figures, why it matters, the response from affected parties, and the next known step.

Preserve important names, numbers, units, dates, prices, qualifications, comparisons, and attribution. Make clear when a figure or benefit is a company claim, estimate, projection, allegation, or independently established fact. Do not add outside knowledge, speculation, generic advice, background absent from the article, or unsupported conclusions. Do not repeat the same fact in multiple bullets, include promotional filler, or copy distinctive phrases longer than necessary.

Write a specific, neutral title that accurately reflects the central event without clickbait. Return exactly one valid JSON object and nothing else, using only the string keys "title" and "summary". The summary must be a single string whose bullets are separated by newline characters and each begin with "• ".`
	payload := map[string]any{"model": c.Model, "messages": []map[string]string{{"role": "system", "content": instruction}, {"role": "user", "content": fmt.Sprintf("source headline: %s\n\noriginal article text:\n%s", title, body)}}, "temperature": 0.2, "stream": false, "response_format": map[string]string{"type": "json_object"}}
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
		return Fields{}, endpointError("summary", res)
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

func endpointError(operation string, res *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<10))
	if err != nil {
		return fmt.Errorf("%s endpoint returned %s; could not read error body: %w", operation, res.Status, err)
	}
	details := []string{fmt.Sprintf("%s endpoint returned %s", operation, res.Status)}
	for _, header := range []string{"Retry-After", "X-Request-ID", "Request-ID", "CF-Ray"} {
		if value := strings.TrimSpace(res.Header.Get(header)); value != "" {
			details = append(details, header+"="+value)
		}
	}
	message := strings.Join(strings.Fields(string(body)), " ")
	if len(message) > 1024 {
		message = message[:1024] + "…"
	}
	if message != "" {
		details = append(details, "body="+fmt.Sprintf("%q", message))
	}
	return errors.New(strings.Join(details, "; "))
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
