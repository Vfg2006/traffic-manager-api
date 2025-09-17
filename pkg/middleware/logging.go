package middleware

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/vfg2006/traffic-manager-api/pkg/log"
)

// RequestIDKey é a chave para armazenar o ID da requisição no contexto
type contextKeyRequestID string

// RequestIDKey é a chave usada para o ID da requisição no contexto
const RequestIDKey contextKeyRequestID = "requestID"

// LoggingMiddleware registra informações sobre cada requisição HTTP
func LoggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Gera um ID de correlação para esta requisição
			ctx, correlationID := log.WithCorrelationID(r.Context())
			r = r.WithContext(ctx)

			// Cria um writer personalizado para capturar o status code
			lrw := newLoggingResponseWriter(w)

			// Registra o início da requisição
			startTime := time.Now()

			isDev := log.IsDevelopment()

			// Em desenvolvimento, usamos um formato mais conciso
			if isDev {
				log.L.WithFields(log.Fields{
					"method": r.Method,
					"path":   r.URL.Path,
				}).Info("→ Iniciando requisição")
			} else {
				// Em produção, registramos todos os detalhes
				log.L.WithFields(log.Fields{
					"correlation_id": correlationID,
					"remote_addr":    r.RemoteAddr,
					"method":         r.Method,
					"path":           r.URL.Path,
					"query":          r.URL.RawQuery,
					"user_agent":     r.UserAgent(),
					"referer":        r.Referer(),
					"content_type":   r.Header.Get("Content-Type"),
					"content_length": r.ContentLength,
				}).Info("Requisição iniciada")
			}

			// Processa a requisição
			next.ServeHTTP(lrw, r)

			// Adiciona campos ao log de resposta
			responseTime := time.Since(startTime)

			// Cria um logger com os campos relevantes
			var logger log.Logger

			if isDev {
				// Formato mais conciso para desenvolvimento
				statusSymbol := "✓"
				if lrw.statusCode >= 400 {
					statusSymbol = "✗"
				}

				logMsg := fmt.Sprintf("%s Completada em %s", statusSymbol, formatDuration(responseTime))

				logger = log.L.WithFields(log.Fields{
					"method":      r.Method,
					"path":        r.URL.Path,
					"status_code": lrw.statusCode,
				})

				if lrw.statusCode >= 500 {
					logger.Error(logMsg)
				} else if lrw.statusCode >= 400 {
					logger.Warn(logMsg)
				} else {
					logger.Info(logMsg)
				}

				// Em desenvolvimento, logar separadamente se a requisição for lenta
				if responseTime > 500*time.Millisecond {
					log.L.Warnf("⚠ Requisição lenta: %s %s (%dms)", r.Method, r.URL.Path, responseTime.Milliseconds())
				}
			} else {
				// Formato completo para produção
				logFields := log.Fields{
					"correlation_id": correlationID,
					"method":         r.Method,
					"path":           r.URL.Path,
					"duration_ms":    responseTime.Milliseconds(),
					"status_code":    lrw.statusCode,
				}

				logger = log.L.WithFields(logFields)

				if lrw.statusCode >= 500 {
					logger.Error("Requisição finalizada com erro")
				} else if lrw.statusCode >= 400 {
					logger.Warn("Requisição finalizada com aviso")
				} else {
					logger.Info("Requisição finalizada com sucesso")
				}

				// Em produção, incluir o aviso de lentidão nos campos
				if responseTime > 500*time.Millisecond {
					log.L.WithFields(logFields).Warnf("Requisição lenta: %s", responseTime)
				}
			}
		})
	}
}

// formatDuration formata a duração de forma humana
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%d µs", d.Microseconds())
	} else if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	} else {
		return fmt.Sprintf("%.2f s", d.Seconds())
	}
}

// loggingResponseWriter é um wrapper para http.ResponseWriter para capturar o status code
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// newLoggingResponseWriter cria um novo loggingResponseWriter
func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Hook para erros não tratados
func LogPanicMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					isDev := log.IsDevelopment()

					// Captura a pilha de chamadas
					stack := make([]byte, 4096)
					stackSize := runtime.Stack(stack, false)
					stackTrace := string(stack[:stackSize])

					if isDev {
						// Formato simplificado para desenvolvimento
						log.L.WithFields(log.Fields{
							"error": err,
							"path":  r.URL.Path,
						}).Error("❌ PANIC na aplicação")

						// Em desenvolvimento, imprimir o stack trace diretamente no console
						fmt.Fprintf(os.Stderr, "\n\n=== STACK TRACE ===\n%s\n=================\n\n", stackTrace)
					} else {
						// Formato completo para produção
						ctx := r.Context()
						correlationID := log.GetCorrelationID(ctx)

						logger := log.L.WithFields(log.Fields{
							"correlation_id": correlationID,
							"panic_error":    err,
							"method":         r.Method,
							"path":           r.URL.Path,
						})

						logger.Error("Erro não tratado na aplicação")
						logger.WithField("stack_trace", stackTrace).Error("Stack trace do erro")
					}

					// Sempre retorna 500 para o cliente
					http.Error(w, "Erro interno no servidor", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
