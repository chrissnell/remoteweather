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
		zapLogger, err = zap.NewDevelopment()
	} else {
		zapLogger, err = zap.NewProduction()
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
		baseLogger, _ = zap.NewProduction()
		log = baseLogger.Sugar()
	}
	return baseLogger
}

// GetSugaredLogger returns the sugared logger instance
func GetSugaredLogger() *zap.SugaredLogger {
	if log == nil {
		// Fallback logger if not initialized
		baseLogger, _ = zap.NewProduction()
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
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Debugf(template, args...)
}

func Debugw(msg string, keysAndValues ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Debugw(msg, keysAndValues...)
}

func Info(args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Info(args...)
}

func Infof(template string, args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Infof(template, args...)
}

func Infow(msg string, keysAndValues ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Infow(msg, keysAndValues...)
}

func Warn(args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Warn(args...)
}

func Warnf(template string, args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Warnf(template, args...)
}

func Warnw(msg string, keysAndValues ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Warnw(msg, keysAndValues...)
}

func Error(args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Error(args...)
}

func Errorf(template string, args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Errorf(template, args...)
}

func Errorw(msg string, keysAndValues ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Errorw(msg, keysAndValues...)
}

func Errorln(args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Error(args...)
}

func Fatal(args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Fatal(args...)
	os.Exit(1)
}

func Fatalf(template string, args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Fatalf(template, args...)
	os.Exit(1)
}
