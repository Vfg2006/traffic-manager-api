package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/insighting"
	"github.com/vfg2006/traffic-manager-api/pkg/log"
	"github.com/vfg2006/traffic-manager-api/pkg/utils"
)

func GetAdAccountsByID(service insighting.CombinedInsighter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := log.ForContext(r.Context())

		id := httprouter.ParamsFromContext(r.Context()).ByName("id")
		logger.WithField("account_id", id).Info("insights: fetching ad account insights by ID")

		startDate, err := utils.ParseDate(r.URL.Query().Get("start_date"))
		if err != nil {
			logger.WithFields(log.Fields{
				"account_id": id,
				"start_date": r.URL.Query().Get("start_date"),
				"error":      err.Error(),
			}).Warn("insights: invalid start_date parameter")

			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		endDate, err := utils.ParseDate(r.URL.Query().Get("end_date"))
		if err != nil {
			logger.WithFields(log.Fields{
				"account_id": id,
				"end_date":   r.URL.Query().Get("end_date"),
				"error":      err.Error(),
			}).Warn("insights: invalid end_date parameter")

			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		filters := &domain.InsigthFilters{
			StartDate: startDate,
			EndDate:   endDate,
		}

		logger.WithFields(log.Fields{
			"account_id": id,
			"start_date": startDate.Format(time.DateOnly),
			"end_date":   endDate.Format(time.DateOnly),
		}).Debug("insights: fetching insights with filters")

		insights, err := service.GetAdAccountsByID(id, filters)
		if err != nil {
			logger.WithFields(log.Fields{
				"account_id": id,
				"start_date": startDate.Format(time.DateOnly),
				"end_date":   endDate.Format(time.DateOnly),
				"error":      err.Error(),
			}).Error("insights: failed to get insights for account")

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Add metrics info to logs if available
		if insights != nil && insights.AdAccountMetrics != nil {
			logger.WithFields(log.Fields{
				"account_id":   id,
				"account_name": insights.AdAccountMetrics.Name,
			}).Info("insights: successfully retrieved account insights")
		} else {
			logger.WithField("account_id", id).Info("insights: no insights data found for account")
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(insights); err != nil {
			logger.WithFields(log.Fields{
				"account_id": id,
				"error":      err.Error(),
			}).Error("insights: failed to encode response")

			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func GetAdAccountReachImpressions(service insighting.CombinedInsighter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := log.ForContext(r.Context())

		id := httprouter.ParamsFromContext(r.Context()).ByName("id")
		logger.WithField("account_id", id).Info("insights: fetching reach and impressions by ID")

		startDate, err := utils.ParseDate(r.URL.Query().Get("start_date"))
		if err != nil {
			logger.WithFields(log.Fields{
				"account_id": id,
				"start_date": r.URL.Query().Get("start_date"),
				"error":      err.Error(),
			}).Warn("insights: invalid start_date parameter")

			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		endDate, err := utils.ParseDate(r.URL.Query().Get("end_date"))
		if err != nil {
			logger.WithFields(log.Fields{
				"account_id": id,
				"end_date":   r.URL.Query().Get("end_date"),
				"error":      err.Error(),
			}).Warn("insights: invalid end_date parameter")

			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		filters := &domain.InsigthFilters{
			StartDate: startDate,
			EndDate:   endDate,
		}

		logger.WithFields(log.Fields{
			"account_id": id,
			"start_date": startDate.Format(time.DateOnly),
			"end_date":   endDate.Format(time.DateOnly),
		}).Debug("insights: fetching reach and impressions with filters")

		response, err := service.GetAdAccountReachImpressions(id, filters)
		if err != nil {
			logger.WithFields(log.Fields{
				"account_id": id,
				"start_date": startDate.Format(time.DateOnly),
				"end_date":   endDate.Format(time.DateOnly),
				"error":      err.Error(),
			}).Error("insights: failed to get reach and impressions for account")

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.WithFields(log.Fields{
				"account_id": id,
				"error":      err.Error(),
			}).Error("insights: failed to encode response")

			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}
