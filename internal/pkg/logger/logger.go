package logger

import (
	"log/slog"
)

type Logger struct {
	*slog.Logger
}

//func New(cfg config.LoggerConfig) (*Logger, error) {
//
//	level := new(slog.LevelVar)
//
//	switch strings.ToLower(cfg.Level) {
//
//	case "debug":
//		level.Set(slog.LevelDebug)
//
//	case "warn":
//		level.Set(slog.LevelWarn)
//
//	case "error":
//		level.Set(slog.LevelError)
//
//	default:
//		level.Set(slog.LevelInfo)
//	}
//
//	var writer io.Writer = os.Stdout
//
//	if cfg.File != "" {
//
//		if err := os.MkdirAll(filepath.Dir(cfg.File), 0755); err != nil {
//			return nil, err
//		}
//
//		file, err := os.OpenFile(
//			cfg.File,
//			os.O_CREATE|os.O_APPEND|os.O_WRONLY,
//			0644,
//		)
//
//		if err != nil {
//			return nil, err
//		}
//
//		writer = io.MultiWriter(
//			os.Stdout,
//			file,
//		)
//	}
//
//	var handler slog.Handler
//
//	switch strings.ToLower(cfg.Format) {
//
//	case "text":
//
//		handler = slog.NewTextHandler(
//			writer,
//			&slog.HandlerOptions{
//				Level: level,
//			},
//		)
//
//	default:
//
//		handler = slog.NewJSONHandler(
//			writer,
//			&slog.HandlerOptions{
//				Level: level,
//			},
//		)
//	}
//
//	return &Logger{
//		Logger: slog.New(handler),
//	}, nil
//}
