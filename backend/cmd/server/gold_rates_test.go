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

func TestFetchGoldRatesMapsUsablePrices(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", request.Method)
		}
		if request.Header.Get("Accept") != "application/json" {
			t.Fatalf("accept = %q", request.Header.Get("Accept"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"success":true,"date":"2026-08-27","time":"23:00","prices":{"SJL1L10":{"name":"SJC 9999","buy":147000000,"sell":150000000,"change_sell":-400000},"SJ9999":{"name":"SJC Ring","buy":146500000,"sell":149500000,"change_sell":0},"DOHNL":{"name":"DOJI Hanoi","buy":147000000,"sell":150000000,"change_sell":400000},"BT9999NTT":{"name":"Bao Tin 9999","buy":148000000,"sell":152000000,"change_sell":-400000}}}`)),
			Header:     make(http.Header),
		}, nil
	})}
	rates, err := fetchGoldRatesWithClient(t.Context(), "https://example.test/prices", client)
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 4 || rates[0].Code != "SJL1L10" || rates[0].Trend != "down" || rates[2].Trend != "up" {
		t.Fatalf("rates = %#v, want four mapped rates", rates)
	}
}
