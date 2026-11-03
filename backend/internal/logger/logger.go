package logger

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"
)

var (
	std  *slog.Logger
	once sync.Once
)

// Init configures the package logger: JSON to stdout plus an optional TCP Logstash sink.
func Init(logstashHost, logstashPort string) {
	once.Do(func() {
		handlers := []slog.Handler{slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})}
		if logstashHost != "" {
			if conn, err := net.DialTimeout("tcp", net.JoinHostPort(logstashHost, logstashPort), 3*time.Second); err == nil {
				handlers = append(handlers, slog.NewJSONHandler(conn, &slog.HandlerOptions{Level: slog.LevelInfo}))
			} else {
				fmt.Fprintf(os.Stderr, "logstash connection failed: %v\n", err)
			}
		}
		std = slog.New(newFanout(handlers...))
	})
	if std == nil {
		std = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
}

func get() *slog.Logger {
	if std == nil {
		Init("", "")
	}
	return std
}

func Info(msg string, args ...any)  { get().Info(msg, args...) }
func Warn(msg string, args ...any)  { get().Warn(msg, args...) }
func Error(msg string, args ...any) { get().Error(msg, args...) }
