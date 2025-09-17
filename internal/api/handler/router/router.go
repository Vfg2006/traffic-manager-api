package router

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

var (
	WithRoutes = func(routes ...Route) ConfigRouter {
		return func(router *Router) {
			router.AddRoutes(routes...)
		}
	}
)

type Route struct {
	Path        string
	Method      string
	Handler     http.Handler
	Middlewares []func(http.Handler) http.Handler // Lista de middlewares específicos para esta rota
}

type Router struct {
	router *httprouter.Router
}

type ConfigRouter func(router *Router)

func New(configs ...ConfigRouter) Router {
	router := &Router{
		router: httprouter.New(),
	}

	for _, config := range configs {
		config(router)
	}

	return *router
}

func (r Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

// AddRoutes adiciona rotas ao router com seus middlewares específicos
func (r Router) AddRoutes(routes ...Route) {
	for _, route := range routes {
		var handler http.Handler = route.Handler

		// Aplicar middlewares específicos da rota, do último para o primeiro
		for i := len(route.Middlewares) - 1; i >= 0; i-- {
			middleware := route.Middlewares[i]
			handler = middleware(handler)
		}

		r.router.Handler(route.Method, route.Path, handler)
	}
}
