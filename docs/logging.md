# Sistema de Logging da API

Este documento descreve o sistema de logging implementado na API traffic Manager.

## Modos de Logging

O sistema de logging possui dois modos diferentes, otimizados para diferentes ambientes:

### Modo de Desenvolvimento (padrão)

* Formato: **Texto colorido**
* Objetivo: **Legibilidade e depuração**
* Características:
  * Mensagens concisas e amigáveis à leitura humana
  * Uso de símbolos (✓, ✗, ❌, ⚠) para facilitar identificação visual
  * Omissão de metadados desnecessários como hostname, PID, etc.
  * Stack trace impresso diretamente no console
  * Duração formatada em unidades apropriadas (µs, ms, s)

### Modo de Produção

* Formato: **JSON estruturado**
* Objetivo: **Agregação e análise automatizada**
* Características:
  * Mensagens completas com todos os metadados
  * Campos estruturados para fácil filtragem
  * Informações de localização precisa (arquivo, linha) 
  * IDs de correlação para rastreamento entre serviços
  * Otimizado para sistemas como ELK Stack, Grafana Loki, etc.

## Como alterar o modo de log

O modo de log é controlado pela variável de ambiente `APP_ENV`:

* **Desenvolvimento** (padrão): quando `APP_ENV` não está definida ou é `development`/`dev`
* **Produção**: quando `APP_ENV` é definida como `production`, `staging` ou qualquer outro valor

```bash
# Para desenvolvimento (logs legíveis)
export APP_ENV=development 

# Para produção (logs estruturados)
export APP_ENV=production
```

## Níveis de Log

O sistema de logging usa os seguintes níveis, em ordem crescente de severidade:

1. **Debug**: Informações detalhadas de debugging, úteis apenas durante o desenvolvimento.
2. **Info**: Confirmações de que as coisas estão funcionando como esperado.
3. **Warn**: Situações inesperadas que não impedem a operação normal, mas devem ser investigadas.
4. **Error**: Erros que impedem a operação normal, mas não causam falha total.
5. **Fatal**: Erros graves que levam ao encerramento da aplicação.
6. **Panic**: Erros catastróficos que causam pânico no sistema.

## Formato do Log em Produção

Os logs em formato JSON incluem os seguintes campos:

- `timestamp`: Data e hora no formato RFC3339.
- `level`: Nível do log (debug, info, warn, error, fatal, panic).
- `message`: Mensagem principal do log.
- `file`: Arquivo e linha onde o log foi gerado.
- `function`: Função que gerou o log.
- `application`: Nome da aplicação.
- `version`: Versão da aplicação.
- `environment`: Ambiente de execução.
- `hostname`: Nome do host onde a aplicação está rodando.
- `pid`: ID do processo.

## Logs de Requisições HTTP

Para requisições HTTP, os seguintes campos são incluídos (em produção):

- `correlation_id`: Identificador único para rastrear uma requisição através de múltiplos serviços.
- `method`: Método HTTP (GET, POST, PUT, DELETE, etc).
- `path`: Caminho da URL.
- `query`: Parâmetros da query string.
- `remote_addr`: Endereço IP do cliente.
- `user_agent`: User agent do cliente.
- `referer`: Referer do cliente.
- `content_type`: Tipo de conteúdo da requisição.
- `content_length`: Tamanho do corpo da requisição.
- `status_code`: Código de status HTTP da resposta.
- `duration_ms`: Tempo de processamento da requisição em milissegundos.

Em desenvolvimento, apenas os campos mais importantes são mostrados.

## Exemplo de saída em Desenvolvimento

```
2025-03-29 15:04:05 INFO  → Iniciando requisição method=GET path=/v1/users
2025-03-29 15:04:05 INFO  ✓ Completada em 45 ms method=GET path=/v1/users status_code=200
2025-03-29 15:04:05 WARN  ⚠ Requisição lenta: GET /v1/accounts (523ms)
2025-03-29 15:04:05 ERROR ❌ PANIC na aplicação error="invalid memory address" path="/v1/reports"

=== STACK TRACE ===
goroutine 1 [running]:
main.example()
        /app/main.go:20 +0x12a
main.main()
        /app/main.go:14 +0x2a
=================
```

## Exemplo de saída em Produção

```json
{"application":"traffic-manager-api","correlation_id":"550e8400-e29b-41d4-a716-446655440000","file":"server.go:47","function":"Server.Run","hostname":"api-server-1","level":"info","message":"Requisição iniciada","method":"GET","path":"/v1/users","pid":12345,"remote_addr":"192.168.1.1:52738","timestamp":"2025-03-29T15:04:05-03:00","user_agent":"Mozilla/5.0","version":"1.0.0"}
```

## Como Usar

### Logging Básico

```go
import "github.com/vfg2006/traffic-manager-api/pkg/log"

// Logging simples
log.L.Info("Iniciando o serviço")

// Com campos adicionais
log.L.WithField("user_id", 123).Info("Usuário autenticado")

// Com múltiplos campos
log.L.WithFields(log.Fields{
    "user_id": 123,
    "action": "login",
}).Info("Usuário realizou login")

// Logging de erros
if err != nil {
    log.L.WithError(err).Error("Falha ao processar requisição")
}
```

### Logging com Contexto (para rastreabilidade)

```go
// Em handlers HTTP, use o contexto da requisição
func MyHandler(w http.ResponseWriter, r *http.Request) {
    logger := log.ForContext(r.Context())
    logger.Info("Processando requisição")
    
    // O ID de correlação será automaticamente incluído
}

// Em funções que recebem contexto
func ProcessData(ctx context.Context, data []byte) error {
    logger := log.ForContext(ctx)
    logger.WithField("data_size", len(data)).Info("Processando dados")
    
    // Resto da função...
    return nil
}
```

## Configuração do Nível de Log

O nível de log pode ser configurado através da variável de ambiente `LOG_LEVEL`. Os valores válidos são:

- `debug`
- `info` (padrão)
- `warn`
- `error`
- `fatal`
- `panic`

Exemplo:

```bash
export LOG_LEVEL=debug
```

## Boas Práticas

1. **Seja específico**: Inclua informações suficientes para entender o contexto.
2. **Não logue dados sensíveis**: Senhas, tokens e informações pessoais nunca devem ser logados.
3. **Use níveis apropriados**: Use `debug` para informações detalhadas, `info` para eventos normais, `warn` para situações inesperadas, e `error` para falhas.
4. **Inclua IDs de correlação**: Sempre propague o contexto com o ID de correlação.
5. **Estruture os logs**: Use campos estruturados em vez de concatenar strings.
6. **Use modo de desenvolvimento** durante a depuração local para facilitar a leitura dos logs. 