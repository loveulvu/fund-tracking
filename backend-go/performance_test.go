package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseFundPerformanceResponseMapsTargetTitles(t *testing.T) {
	body := []byte(`{
		"Success": true,
		"Datas": [
			{"title":"Z","syl":"1.23"},
			{"title":"Y","syl":"2.34"},
			{"title":"3Y","syl":"3.45"},
			{"title":"6Y","syl":"4.56"},
			{"title":"1N","syl":"5.67"},
			{"title":"3N","syl":"6.78"}
		]
	}`)

	performance, err := parseFundPerformanceResponse(body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := map[string]float64{
		"week_growth":        1.23,
		"month_growth":       2.34,
		"three_month_growth": 3.45,
		"six_month_growth":   4.56,
		"year_growth":        5.67,
		"three_year_growth":  6.78,
	}
	for field, value := range expected {
		if performance[field] != value {
			t.Fatalf("expected %s=%v, got %v", field, value, performance[field])
		}
	}
}

func TestParsePerformanceValue(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  float64
		ok    bool
	}{
		{name: "positive string", value: "105.49", want: 105.49, ok: true},
		{name: "negative string", value: "-27.34", want: -27.34, ok: true},
		{name: "zero string", value: "0", want: 0, ok: true},
		{name: "empty string", value: "", ok: false},
		{name: "dash placeholder", value: "--", ok: false},
		{name: "nil", value: nil, ok: false},
		{name: "invalid number", value: "not-a-number", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parsePerformanceValue(tt.value)
			if ok != tt.ok {
				t.Fatalf("expected ok=%v, got %v", tt.ok, ok)
			}
			if ok && got != tt.want {
				t.Fatalf("expected value %v, got %v", tt.want, got)
			}
		})
	}
}

func TestParseFundPerformanceResponseSkipsMissingInvalidAndNonTargetValues(t *testing.T) {
	body := []byte(`{
		"Success": true,
		"Datas": [
			{"title":"Z","syl":""},
			{"title":"Y","syl":"--"},
			{"title":"3Y","syl":null},
			{"title":"6Y","syl":"not-a-number"},
			{"title":"1N"},
			{"title":"2N","syl":"9.99"},
			{"title":"5N","syl":"8.88"},
			{"title":"JN","syl":"7.77"},
			{"title":"LN","syl":"6.66"}
		]
	}`)

	performance, err := parseFundPerformanceResponse(body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(performance) != 0 {
		t.Fatalf("expected no valid performance fields, got %#v", performance)
	}
}

func TestBuildFundPerformanceUpdateAllowsOnlyTargetFields(t *testing.T) {
	update := buildFundPerformanceUpdate(map[string]float64{
		"week_growth":          1.23,
		"month_growth":         0,
		"unknown_growth":       9.99,
		"performanceUpdatedAt": 123,
	})

	if update["week_growth"] != 1.23 {
		t.Fatalf("expected week_growth to be set, got %v", update["week_growth"])
	}
	if update["month_growth"] != float64(0) {
		t.Fatalf("expected month_growth zero to be preserved, got %v", update["month_growth"])
	}
	if _, ok := update["unknown_growth"]; ok {
		t.Fatal("expected unknown_growth to be ignored")
	}
	if _, ok := update["performanceUpdatedAt"]; !ok {
		t.Fatal("expected performanceUpdatedAt to be set")
	}
}

func TestBuildFundPerformanceUpdateSkipsEmptyMap(t *testing.T) {
	update := buildFundPerformanceUpdate(map[string]float64{})
	if len(update) != 0 {
		t.Fatalf("expected empty update, got %#v", update)
	}
}

func TestPerformanceFundsHandlerRequiresUpdateKeyHeader(t *testing.T) {
	t.Setenv("UPDATE_API_KEY", "test-update-key")

	request := httptest.NewRequest(http.MethodPost, "/api/funds/performance", nil)
	response := httptest.NewRecorder()

	performanceFundsHandler(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestPerformanceFundsHandlerRejectsQueryKey(t *testing.T) {
	t.Setenv("UPDATE_API_KEY", "test-update-key")

	request := httptest.NewRequest(http.MethodPost, "/api/funds/performance?key=test-update-key", nil)
	response := httptest.NewRecorder()

	performanceFundsHandler(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestPerformanceFundsHandlerFailsClosedWhenUpdateKeyMissing(t *testing.T) {
	t.Setenv("UPDATE_API_KEY", "")

	request := httptest.NewRequest(http.MethodPost, "/api/funds/performance", nil)
	request.Header.Set("X-Update-Key", "test-update-key")
	response := httptest.NewRecorder()

	performanceFundsHandler(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestPerformanceKeyHelperAcceptsConfiguredHeader(t *testing.T) {
	t.Setenv("UPDATE_API_KEY", "test-update-key")

	request := httptest.NewRequest(http.MethodPost, "/api/funds/performance", nil)
	request.Header.Set("X-Update-Key", "test-update-key")

	if !requireUpdateAPIKeyHeader(request) {
		t.Fatal("expected matching X-Update-Key to pass")
	}
}
