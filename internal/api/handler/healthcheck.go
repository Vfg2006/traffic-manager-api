package handler

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

func HealthcheckHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(time.Now().String()))
		if err != nil {
			logrus.Println("error on respond to liveness:", err)
			logrus.WithError(err).Warn("error responding to healthcheck")
		}
	})
}
