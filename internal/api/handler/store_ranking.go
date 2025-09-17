package handler

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/ranking"
	"github.com/vfg2006/traffic-manager-api/pkg/apiErrors"
)

// GetStoreRanking retorna o ranking das lojas por receita de redes sociais
func GetStoreRanking(service ranking.RankingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Buscar o ranking das lojas
		ranking, err := service.GetStoreRanking()
		if err != nil {
			logrus.Error("Erro ao buscar ranking das lojas:", err)
			apiErrors.WriteError(w, apiErrors.ErrDatabaseOperation, "Erro ao buscar ranking das lojas", nil)
			return
		}

		if ranking == nil {
			apiErrors.WriteError(w, apiErrors.ErrUserNotFound, "Nenhum ranking encontrado", nil)
			return
		}

		// Enviar resposta
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(ranking)
		if err != nil {
			logrus.Error("Erro ao enviar resposta do ranking:", err)
			apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao enviar resposta", nil)
			return
		}
	}
}
