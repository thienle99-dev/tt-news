package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const goldRatesPageURL = "https://baotinmanhhai.vn/bang-gia-vang"

type goldRate struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	BuyPrice    float64 `json:"buy_price"`
	SellPrice   float64 `json:"sell_price"`
	Unit        string  `json:"unit"`
	Trend       string  `json:"trend"`
	TrendValue  string  `json:"trend_value"`
	LastUpdated string  `json:"last_updated"`
}

type goldRatesQueryResponse struct {
	Data struct {
		GoldRates struct {
			Items []goldRate `json:"items"`
		} `json:"goldRates"`
	} `json:"data"`
}

func (s *server) goldRatesHandler(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	s.goldRatesMu.Lock()
	if now.Before(s.goldRatesUntil) {
		rates := append([]goldRate(nil), s.goldRates...)
		s.goldRatesMu.Unlock()
		jsonOut(w, http.StatusOK, rates)
		return
	}
	s.goldRatesMu.Unlock()

	rates, err := fetchGoldRates(r.Context(), s.cfg.GoldRatesURL)
	if err != nil {
		jsonErr(w, http.StatusBadGateway, "could not load gold rates")
		return
	}
	s.goldRatesMu.Lock()
	s.goldRates = append([]goldRate(nil), rates...)
	s.goldRatesUntil = now.Add(time.Minute)
	s.goldRatesMu.Unlock()
	jsonOut(w, http.StatusOK, rates)
}

func fetchGoldRates(ctx context.Context, endpoint string) ([]goldRate, error) {
	return fetchGoldRatesWithClient(ctx, endpoint, &http.Client{Timeout: 12 * time.Second})
}

func fetchGoldRatesWithClient(ctx context.Context, endpoint string, client *http.Client) ([]goldRate, error) {
	payload := map[string]any{
		"operationName": "GetGoldRates",
		"query": `query GetGoldRates($limit: Int, $rate_codes: [String!]) {
  goldRates(limit: $limit, rate_codes: $rate_codes) {
    items {
      code name buy_price sell_price unit trend trend_value last_updated
    }
  }
}`,
		"variables": map[string]any{
			"limit":      4,
			"rate_codes": []string{"KGBG", "KGB", "9999", "999"},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/graphql-response+json, application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://baotinmanhhai.vn")
	req.Header.Set("Referer", goldRatesPageURL)
	req.Header.Set("User-Agent", "TelegramNewsRSSBot/1.0")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("gold rates endpoint returned %s", res.Status)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var response goldRatesQueryResponse
	if err = json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	rates := make([]goldRate, 0, len(response.Data.GoldRates.Items))
	for _, rate := range response.Data.GoldRates.Items {
		rate.Name, rate.Unit = strings.TrimSpace(rate.Name), strings.TrimSpace(rate.Unit)
		if rate.Name == "" || rate.BuyPrice <= 0 || rate.SellPrice <= rate.BuyPrice {
			continue
		}
		rates = append(rates, rate)
	}
	if len(rates) == 0 {
		return nil, fmt.Errorf("gold rates endpoint returned no usable rates")
	}
	return rates, nil
}
