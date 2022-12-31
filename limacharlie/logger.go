package limacharlie

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/google/uuid"
)

// LCLogger is the interface for limacharlie logging
type LCLogger interface {
	Fatal(msg string)
	Error(msg string)
	Warn(msg string)
	Info(msg string)
	Debug(msg string)
	Trace(msg string)
}

// LCLoggerEmpty does not actually log anything
type LCLoggerEmpty struct{}

// Fatal empty stub for logging interface
func (l *LCLoggerEmpty) Fatal(msg string) {}

// Error empty stub for logging interface
func (l *LCLoggerEmpty) Error(msg string) {}

// Warn empty stub for logging interface
func (l *LCLoggerEmpty) Warn(msg string) {}

// Info empty stub for logging interface
func (l *LCLoggerEmpty) Info(msg string) {}

// Debug empty stub for logging interface
func (l *LCLoggerEmpty) Debug(msg string) {}

// Trace empty stub for logging interface
func (l *LCLoggerEmpty) Trace(msg string) {}

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

// LCLoggerGCP implements the logging interface with GCP structured logging
type gcpLogTime struct {
	Seconds int64 `json:"seconds"`
	Nanos   int   `json:"nanos"`
}

type gcpLogLine struct {
	Message    string                 `json:"message"`
	Timestamp  gcpLogTime             `json:"timestamp"`
	CustomData map[string]interface{} `json:"ext_data,omitempty"`
	InstanceID string                 `json:"instance_id,omitempty"`
	Severity   string                 `json:"severity,omitempty"`
}

var newLine = []byte{'\n'}

type LCLoggerGCP struct {
	m          sync.Mutex
	instanceID string
	once       sync.Once
}

func (l *LCLoggerGCP) logToStdout(msg string, severity string) {
	line := gcpLogLine{
		Message:    string(msg),
		InstanceID: l.instanceID,
		Severity:   severity,
	}
	now := time.Now()
	line.Timestamp.Seconds = now.Unix()
	line.Timestamp.Nanos = now.Nanosecond()
	b, _ := json.Marshal(&line)
	l.m.Lock()
	defer l.m.Unlock()
	os.Stdout.Write(b)
	os.Stderr.Write(newLine)
}

func (l *LCLoggerGCP) logToStderr(msg string, severity string) {
	line := gcpLogLine{
		Message:    string(msg),
		InstanceID: l.instanceID,
		Severity:   severity,
	}
	now := time.Now()
	line.Timestamp.Seconds = now.Unix()
	line.Timestamp.Nanos = now.Nanosecond()
	b, _ := json.Marshal(&line)
	l.m.Lock()
	defer l.m.Unlock()
	os.Stderr.Write(b)
	os.Stderr.Write(newLine)
}

func (l *LCLoggerGCP) init() {
	l.instanceID = uuid.NewString()
}

// Fatal see GCP logger fatal function
func (l *LCLoggerGCP) Fatal(msg string) {
	l.once.Do(l.init)
	l.logToStderr(msg, "CRITICAL")
}

// Error see GCP logger error function
func (l *LCLoggerGCP) Error(msg string) {
	l.once.Do(l.init)
	l.logToStderr(msg, "ERROR")
}

// Warn see GCP logger warn function
func (l *LCLoggerGCP) Warn(msg string) {
	l.once.Do(l.init)
	l.logToStdout(msg, "WARNING")
}

// Info see GCP logger info function
func (l *LCLoggerGCP) Info(msg string) {
	l.once.Do(l.init)
	l.logToStdout(msg, "INFO")
}

// Debug see GCP logger debug function
func (l *LCLoggerGCP) Debug(msg string) {
	l.once.Do(l.init)
	l.logToStdout(msg, "DEBUG")
}

// Trace see GCP logger trace function
func (l *LCLoggerGCP) Trace(msg string) {
	l.once.Do(l.init)
	l.logToStdout(msg, "DEFAULT")
}
