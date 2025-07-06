// Package log provides centralized logging functionality using zap logger.
package log

import (
	"fmt"
	"os"

	"go.uber.org/zap"
)

var log *zap.SugaredLogger
var baseLogger *zap.Logger

// Init initializes the package-level logger
func Init(debug bool) error {
	var zapLogger *zap.Logger
	var err error

	if debug {
		zapLogger, err = zap.NewDevelopment(zap.AddCallerSkip(1))
	} else {
		zapLogger, err = zap.NewProduction(zap.AddCallerSkip(1))
	}
	if err != nil {
		return fmt.Errorf("can't initialize zap logger: %v", err)
	}

	baseLogger = zapLogger
	log = zapLogger.Sugar()
	return nil
}

// GetZapLogger returns the base zap logger for cases where it's needed (like GORM)
func GetZapLogger() *zap.Logger {
	if baseLogger == nil {
		// Fallback logger if not initialized
		baseLogger, _ = zap.NewProduction(zap.AddCallerSkip(1))
		log = baseLogger.Sugar()
	}
	return baseLogger
}

// GetSugaredLogger returns the sugared logger instance
func GetSugaredLogger() *zap.SugaredLogger {
	if log == nil {
		// Fallback logger if not initialized
		baseLogger, _ = zap.NewProduction(zap.AddCallerSkip(1))
		log = baseLogger.Sugar()
	}
	return log
}

// Sync flushes any buffered log entries
func Sync() {
	if log != nil {
		log.Sync()
	}
}

// Package-level convenience functions
func Debug(args ...interface{}) {
	log.Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	log.Debugf(template, args...)
}

func Debugw(msg string, keysAndValues ...interface{}) {
	log.Debugw(msg, keysAndValues...)
}

func Info(args ...interface{}) {
	log.Info(args...)
}

func Infof(template string, args ...interface{}) {
	log.Infof(template, args...)
}

func Infow(msg string, keysAndValues ...interface{}) {
	log.Infow(msg, keysAndValues...)
}

func Warn(args ...interface{}) {
	log.Warn(args...)
}

func Warnf(template string, args ...interface{}) {
	log.Warnf(template, args...)
}

func Warnw(msg string, keysAndValues ...interface{}) {
	log.Warnw(msg, keysAndValues...)
}

func Error(args ...interface{}) {
	log.Error(args...)
}

func Errorf(template string, args ...interface{}) {
	log.Errorf(template, args...)
}

func Errorw(msg string, keysAndValues ...interface{}) {
	log.Errorw(msg, keysAndValues...)
}

func Errorln(args ...interface{}) {
	log.Error(args...)
}

func Fatal(args ...interface{}) {
	log.Fatal(args...)
	os.Exit(1)
}

func Fatalf(template string, args ...interface{}) {
	log.Fatalf(template, args...)
	os.Exit(1)
}
