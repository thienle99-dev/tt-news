package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const goldRatesURL = "https://www.vang.today/api/prices"

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

type vangTodayResponse struct {
	Success bool                      `json:"success"`
	Date    string                    `json:"date"`
	Time    string                    `json:"time"`
	Prices  map[string]vangTodayPrice `json:"prices"`
}

type vangTodayPrice struct {
	Name       string  `json:"name"`
	Buy        float64 `json:"buy"`
	Sell       float64 `json:"sell"`
	ChangeSell float64 `json:"change_sell"`
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
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
	var response vangTodayResponse
	if err = json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("gold rates endpoint reported failure")
	}
	updated := strings.TrimSpace(response.Date + " " + response.Time)
	rates := make([]goldRate, 0, 4)
	for _, code := range []string{"SJL1L10", "SJ9999", "DOHNL", "BT9999NTT"} {
		price, ok := response.Prices[code]
		if !ok || strings.TrimSpace(price.Name) == "" || price.Buy <= 0 || price.Sell <= price.Buy {
			continue
		}
		trend := "neutral"
		if price.ChangeSell > 0 {
			trend = "up"
		} else if price.ChangeSell < 0 {
			trend = "down"
		}
		rates = append(rates, goldRate{
			Code: code, Name: strings.TrimSpace(price.Name), BuyPrice: price.Buy, SellPrice: price.Sell,
			Unit: "VND/lượng", Trend: trend, TrendValue: fmt.Sprintf("%+.0f", price.ChangeSell), LastUpdated: updated,
		})
	}
	if len(rates) == 0 {
		return nil, fmt.Errorf("gold rates endpoint returned no usable rates")
	}
	return rates, nil
}
