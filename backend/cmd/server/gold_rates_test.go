package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestFetchGoldRatesFiltersUnavailablePrices(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", request.Method)
		}
		if request.Header.Get("Origin") != "https://baotinmanhhai.vn" {
			t.Fatalf("origin = %q", request.Header.Get("Origin"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{"data":{"goldRates":{"items":[{"code":"KGBG","name":"Kim Gia Bảo Gift 24K","buy_price":15300000,"sell_price":15700000,"unit":"VND/1 chỉ","trend":"down","trend_value":"-120.000","last_updated":"2026-08-27 14:40:05.0"},{"code":"BT24K","name":"Unavailable sale price","buy_price":14700000,"sell_price":1}]}}}`)),
			Header: make(http.Header),
		}, nil
	})}
	rates, err := fetchGoldRatesWithClient(t.Context(), "https://example.test/graphql", client)
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 1 || rates[0].Code != "KGBG" {
		t.Fatalf("rates = %#v, want one usable rate", rates)
	}
}
