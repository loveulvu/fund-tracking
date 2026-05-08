package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

type Fund struct {
	FundCode         string  `json:"fund_code"`
	FundName         string  `json:"fund_name"`
	NetValue         float64 `json:"net_value"`
	DayGrowth        float64 `json:"day_growth"`
	WeekGrowth       float64 `json:"week_growth"`
	MonthGrowth      float64 `json:"month_growth"`
	ThreeMonthGrowth float64 `json:"three_month_growth"`
	SixMonthGrowth   float64 `json:"six_month_growth"`
	YearGrowth       float64 `json:"year_growth"`
	ThreeYearGrowth  float64 `json:"three_year_growth"`
	FundType         string  `json:"fund_type"`
	FundCompany      string  `json:"fund_company"`
	FundManager      string  `json:"fund_manager"`
	FundScale        string  `json:"fund_scale"`
	NetValueDate     string  `json:"net_value_date"`
	UpdateTime       string  `json:"update_time"`
	IsSeed           bool    `json:"is_seed"`
	IsWatched        bool    `json:"is_watched"`
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
func loadFundsFromFile() ([]Fund, error) {
	data, err := os.ReadFile("data/funds.json")
	if err != nil {
		return nil, err
	}
	var funds []Fund
	if err := json.Unmarshal(data, &funds); err != nil {
		return nil, err
	}
	return funds, nil
}
func findFundByCode(code string) (Fund, bool, error) {
	funds, err := loadFundsFromFile()
	if err != nil {
		return Fund{}, false, err
	}
	for _, fund := range funds {
		if fund.FundCode == code {
			return fund, true, nil
		}
	}
	return Fund{}, false, nil
}
func fundsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	funds, err := loadFundsFromFile()
	if err != nil {
		http.Error(w, "Failed to load funds", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	if err := json.NewEncoder(w).Encode(funds); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
func fundDetailHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	code := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/fund/"), "/")
	if code == "" {
		http.Error(w, "Fund code is required", http.StatusBadRequest)
		return
	}
	fund, ok, err := findFundByCode(code)
	if err != nil {
		http.Error(w, "Failed to find fund", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Fund not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	if err := json.NewEncoder(w).Encode(fund); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

}
func main() {
	http.HandleFunc("/api/funds", fundsHandler)
	http.HandleFunc("/api/fund/", fundDetailHandler)
	log.Println("Server is running on http://127.0.0.1:8081")
	err := http.ListenAndServe("127.0.0.1:8081", nil)
	if err != nil {
		log.Fatal(err)
	}
}
