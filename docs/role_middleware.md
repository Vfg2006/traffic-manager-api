# Middleware de Controle de Acesso por Role

Este documento descreve como usar o middleware de controle de acesso baseado em roles para proteger rotas na API.

## Conceitos Básicos

O sistema de controle de acesso permite restringir o acesso a rotas específicas com base no role (papel/função) do usuário autenticado. Atualmente, existem os seguintes roles:

- **RoleAdmin (ID: 1)**: Administradores do sistema com acesso total
- **RoleClient (ID: 3)**: Usuários comuns com acesso limitado

## Como Aplicar o Middleware a Rotas

### Método 1: Aplicar diretamente na definição da rota

```go
{
    Path:        "/v1/alguma-rota",
    Method:      http.MethodGet,
    Handler:     AlgumHandler(service),
    Middlewares: []func(http.Handler) http.Handler{middleware.AdminOnly()},
}
```

### Método 2: Usar os helpers para aplicar a múltiplas rotas

```go
// Criar rotas sem middleware inicialmente
rotas := []router.Route{
    {
        Path:    "/v1/exemplo/rota1",
        Method:  http.MethodGet,
        Handler: Handler1(service),
    },
    {
        Path:    "/v1/exemplo/rota2",
        Method:  http.MethodGet,
        Handler: Handler2(service),
    },
}

// Aplicar middleware a todas as rotas de uma vez
rotas = middleware.AdminOnlyRoutes(rotas)
```

## Middleware Disponíveis

### 1. `AdminOnly()`

Restringe o acesso apenas para administradores (role ID 1).

```go
Middlewares: []func(http.Handler) http.Handler{middleware.AdminOnly()},
```

### 2. `ClientOrAdmin()`

Permite acesso tanto para administradores (role ID 1) quanto para clientes (role ID 3).

```go
Middlewares: []func(http.Handler) http.Handler{middleware.ClientOrAdmin()},
```

### 3. `RoleMiddleware(allowedRoles []int)`

Permite definir manualmente quais roles têm acesso à rota.

```go
// Permite acesso para roles com IDs 1, 2 e 5
Middlewares: []func(http.Handler) http.Handler{middleware.RoleMiddleware([]int{1, 2, 5})},
```

## Funções Auxiliares

O sistema também oferece funções auxiliares para aplicar middlewares a múltiplas rotas de uma vez:

- `AdminOnlyRoutes(routes []router.Route) []router.Route`: Aplica o middleware AdminOnly a todas as rotas fornecidas
- `ClientOrAdminRoutes(routes []router.Route) []router.Route`: Aplica o middleware ClientOrAdmin a todas as rotas fornecidas
- `CustomRoleRoutes(routes []router.Route, allowedRoles []int) []router.Route`: Aplica um middleware personalizado com roles específicos

## Exemplos

Veja o arquivo `internal/api/handler/routes_example.go` para exemplos completos de como usar os middlewares de role.

## Fluxo de Controle de Acesso

1. O usuário se autentica através do endpoint `/v1/login`
2. O token JWT retornado contém o role do usuário
3. Ao acessar rotas protegidas, o middleware `AuthMiddleware` valida o token e coloca as claims no contexto
4. O middleware `RoleMiddleware` verifica se o role do usuário está na lista de roles permitidos
5. Se estiver permitido, a requisição é processada. Caso contrário, retorna erro 403 Forbidden 