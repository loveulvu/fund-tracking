package main

import (
	"encoding/json"
	"log"
	"net/http"
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
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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
	funds := []Fund{
		{
			FundCode:         "008540",
			FundName:         "示例基金Acc",
			NetValue:         1.2345,
			DayGrowth:        0.56,
			WeekGrowth:       1.23,
			MonthGrowth:      3.45,
			ThreeMonthGrowth: 5.67,
			SixMonthGrowth:   8.91,
			YearGrowth:       12.34,
			ThreeYearGrowth:  35.67,
			FundType:         "混合型",
			FundCompany:      "示例基金公司",
			FundManager:      "张三",
			FundScale:        "120.5亿",
			NetValueDate:     "2026-05-06",
			UpdateTime:       "2026-05-07 12:00:00",
			IsSeed:           true,
			IsWatched:        false,
		},
		{
			FundCode:         "000001",
			FundName:         "示例基金B",
			NetValue:         2.3456,
			DayGrowth:        -0.32,
			WeekGrowth:       0.88,
			MonthGrowth:      2.11,
			ThreeMonthGrowth: 4.56,
			SixMonthGrowth:   6.78,
			YearGrowth:       9.87,
			ThreeYearGrowth:  28.90,
			FundType:         "股票型",
			FundCompany:      "测试基金公司",
			FundManager:      "李四",
			FundScale:        "88.2亿",
			NetValueDate:     "2026-05-06",
			UpdateTime:       "2026-05-07 12:00:00",
			IsSeed:           false,
			IsWatched:        true,
		},
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	if err := json.NewEncoder(w).Encode(funds); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
func main() {
	http.HandleFunc("/api/funds", fundsHandler)
	log.Println("Server is running on http://127.0.0.1:8081")
	err := http.ListenAndServe("127.0.0.1:8081", nil)
	if err != nil {
		log.Fatal(err)
	}
}
