package middleware

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"github.com/vfg2006/traffic-manager-api/pkg/apiErrors"
)

// Constantes para identificar os roles
const (
	RoleAdmin      = 1
	RoleSupervisor = 2
	RoleClient     = 3
	// Adicione outros roles conforme necessário
)

// RoleMiddleware cria um middleware que restringe o acesso com base nos roles
// allowedRoles é um array de IDs de roles que têm permissão para acessar a rota
func RoleMiddleware(allowedRoles []int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Obter claims do usuário do contexto
			userClaims, ok := r.Context().Value(ContextKeyUser).(*domain.Claims)

			if !ok {
				logrus.Warning("Tentativa de acesso sem autenticação")
				apiErrors.WriteError(w, apiErrors.ErrInvalidToken, "Usuário não autenticado", nil)
				return
			}

			// Verificar se o role do usuário está na lista de roles permitidos
			isAllowed := false
			for _, role := range allowedRoles {
				if userClaims.UserRoleID == role {
					isAllowed = true
					break
				}
			}

			if !isAllowed {
				logrus.Warningf("Acesso negado para usuário ID=%d, Role=%d", userClaims.UserID, userClaims.UserRoleID)
				apiErrors.WriteError(w, apiErrors.ErrInsufficientPrivilege, "Você não tem permissão para acessar este recurso", nil)
				return
			}

			// Se tiver permissão, continua para o próximo handler
			next.ServeHTTP(w, r)
		})
	}
}

// AdminOnly é um middleware que permite acesso apenas para administradores
func AdminOnly() func(http.Handler) http.Handler {
	return RoleMiddleware([]int{RoleAdmin})
}

// AdminOrSupervisor é um middleware que permite acesso para administradores e gerentes
func AdminOrSupervisor() func(http.Handler) http.Handler {
	return RoleMiddleware([]int{RoleAdmin, RoleSupervisor})
}

// ClientOrAdmin é um middleware que permite acesso para clientes e administradores
func AllRoles() func(http.Handler) http.Handler {
	return RoleMiddleware([]int{RoleAdmin, RoleSupervisor, RoleClient})
}
