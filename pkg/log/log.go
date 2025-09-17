package log

import (
	"context"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Fields é um alias para logrus.Fields
type Fields logrus.Fields

// Logger é uma interface que define os métodos de log
type Logger interface {
	WithField(key string, value interface{}) Logger
	WithFields(fields Fields) Logger
	WithError(err error) Logger
	WithContext(ctx context.Context) Logger

	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Panic(args ...interface{})
	Panicf(format string, args ...interface{})
}

// contextKey para armazenar o ID de correlação no contexto
type contextKey string

// CorrelationIDKey é a chave para armazenar o ID de correlação no contexto
const CorrelationIDKey contextKey = "correlation_id"
const correlationIDField = "correlation_id"

// logger implementa a interface Logger e encapsula logrus
type logger struct {
	entry *logrus.Entry
}

// L é uma instância global de Logger para uso direto
var L Logger = &logger{entry: logrus.NewEntry(logrus.StandardLogger())}

// IsDevelopment retorna verdadeiro se estamos em ambiente de desenvolvimento
func IsDevelopment() bool {
	env := os.Getenv("APP_ENV")
	return env == "" || env == "development" || env == "dev"
}

// SetupTestLogger configura um logger simplificado para testes
func SetupTestLogger() {
	// Formato de texto para testes - mais legível e compacto
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:    false,
		DisableColors:    false,
		DisableTimestamp: false,
		PadLevelText:     true,
	})

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetReportCaller(false)

	// Redefine a instância global
	L = &logger{entry: logrus.NewEntry(logrus.StandardLogger())}
}

// WithField adiciona um único campo ao Logger
func (l *logger) WithField(key string, value interface{}) Logger {
	// Em desenvolvimento, omitimos campos de rastreabilidade para logs mais limpos,
	// a menos que seja o campo correlation_id
	if IsDevelopment() && key != correlationIDField &&
		key != "method" && key != "path" && key != "status_code" &&
		key != "duration_ms" && key != "error" {
		return l
	}
	return &logger{entry: l.entry.WithField(key, value)}
}

// WithFields adiciona múltiplos campos ao Logger
func (l *logger) WithFields(fields Fields) Logger {
	// Em desenvolvimento, filtramos campos irrelevantes
	if IsDevelopment() {
		relevantFields := make(logrus.Fields)
		for k, v := range fields {
			// Mantém apenas campos importantes para depuração
			if k == correlationIDField || k == "method" || k == "path" ||
				k == "status_code" || k == "duration_ms" || k == "error" ||
				strings.HasPrefix(k, "user_") {
				relevantFields[k] = v
			}
		}
		if len(relevantFields) == 0 {
			return l
		}
		return &logger{entry: l.entry.WithFields(relevantFields)}
	}

	return &logger{entry: l.entry.WithFields(logrus.Fields(fields))}
}

// WithError adiciona um erro ao Logger
func (l *logger) WithError(err error) Logger {
	return &logger{entry: l.entry.WithError(err)}
}

// WithContext extrai informações do contexto para o Logger
func (l *logger) WithContext(ctx context.Context) Logger {
	if ctx == nil {
		return l
	}

	// Extrai o ID de correlação do contexto se existir
	if correlationID, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return l.WithField(correlationIDField, correlationID)
	}

	return l
}

// Debug loga uma mensagem no nível debug
func (l *logger) Debug(args ...interface{}) {
	l.entry.Debug(args...)
}

// Debugf loga uma mensagem formatada no nível debug
func (l *logger) Debugf(format string, args ...interface{}) {
	l.entry.Debugf(format, args...)
}

// Info loga uma mensagem no nível info
func (l *logger) Info(args ...interface{}) {
	l.entry.Info(args...)
}

// Infof loga uma mensagem formatada no nível info
func (l *logger) Infof(format string, args ...interface{}) {
	l.entry.Infof(format, args...)
}

// Warn loga uma mensagem no nível warning
func (l *logger) Warn(args ...interface{}) {
	l.entry.Warn(args...)
}

// Warnf loga uma mensagem formatada no nível warning
func (l *logger) Warnf(format string, args ...interface{}) {
	l.entry.Warnf(format, args...)
}

// Error loga uma mensagem no nível error
func (l *logger) Error(args ...interface{}) {
	l.entry.Error(args...)
}

// Errorf loga uma mensagem formatada no nível error
func (l *logger) Errorf(format string, args ...interface{}) {
	l.entry.Errorf(format, args...)
}

// Fatal loga uma mensagem no nível fatal
func (l *logger) Fatal(args ...interface{}) {
	l.entry.Fatal(args...)
}

// Fatalf loga uma mensagem formatada no nível fatal
func (l *logger) Fatalf(format string, args ...interface{}) {
	l.entry.Fatalf(format, args...)
}

// Panic loga uma mensagem no nível panic
func (l *logger) Panic(args ...interface{}) {
	l.entry.Panic(args...)
}

// Panicf loga uma mensagem formatada no nível panic
func (l *logger) Panicf(format string, args ...interface{}) {
	l.entry.Panicf(format, args...)
}

// WithCorrelationID adiciona um ID de correlação ao contexto
func WithCorrelationID(ctx context.Context) (context.Context, string) {
	correlationID := uuid.New().String()
	return context.WithValue(ctx, CorrelationIDKey, correlationID), correlationID
}

// GetCorrelationID obtém o ID de correlação do contexto
func GetCorrelationID(ctx context.Context) string {
	if correlationID, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return correlationID
	}
	return ""
}

// ForContext cria um logger com o ID de correlação do contexto
func ForContext(ctx context.Context) Logger {
	return L.WithContext(ctx)
}
