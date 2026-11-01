package main

import "net/http"

func newStatisticsHandler(
	store urlRepository,
	reader clickStatisticsReader,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		record, err := store.get(r.Context(), r.PathValue("code"))
		if err != nil {
			writeServiceError(w, err)
			return
		}

		stats, err := reader.getClickStatistics(r.Context(), record.ID)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		type dailyResponse struct {
			Date   string `json:"date"`
			Clicks int64  `json:"clicks"`
		}

		daily := make([]dailyResponse, 0, len(stats.Days))
		for _, day := range stats.Days {
			daily = append(daily, dailyResponse{
				Date:   day.Date,
				Clicks: day.Count,
			})
		}

		response := struct {
			Code        string          `json:"code"`
			TotalClicks int64           `json:"total_clicks"`
			Daily       []dailyResponse `json:"daily"`
		}{
			Code:        record.Code,
			TotalClicks: stats.Total,
			Daily:       daily,
		}

		writeJSON(w, http.StatusOK, response)
	}
}
