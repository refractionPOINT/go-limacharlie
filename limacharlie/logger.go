package limacharlie

import "github.com/rs/zerolog"

// LCLogger is the interface for limacharlie logging
type LCLogger interface {
	Fatal(msg string)
	Error(msg string)
	Warn(msg string)
	Info(msg string)
	Debug(msg string)
	Trace(msg string)
}

// LCLoggerZerolog implements the logging interface with zerolog
type LCLoggerZerolog struct {
	l zerolog.Logger
}

// Fatal see zerolog logger fatal function
func (l *LCLoggerZerolog) Fatal(msg string) {
	l.l.Fatal().Msg(msg)
}

// Error see zerolog logger error function
func (l *LCLoggerZerolog) Error(msg string) {
	l.l.Error().Msg(msg)
}

// Warn see zerolog logger warn function
func (l *LCLoggerZerolog) Warn(msg string) {
	l.l.Warn().Msg(msg)
}

// Info see zerolog logger info function
func (l *LCLoggerZerolog) Info(msg string) {
	l.l.Info().Msg(msg)
}

// Debug see zerolog logger debug function
func (l *LCLoggerZerolog) Debug(msg string) {
	l.l.Debug().Msg(msg)
}

// Trace see zerolog logger trace function
func (l *LCLoggerZerolog) Trace(msg string) {
	l.l.Trace().Msg(msg)
}
