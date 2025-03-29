package log

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/eli-yip/rss-zero/config"
)

var DefaultLogger *zap.Logger

// NewZapLogger returns a new zap logger.
// To enable debug mode, add DEBUG=true to .env file.
// It will use production config otherwise.
func NewZapLogger() *zap.Logger {
	lumberjacklogger := &lumberjack.Logger{
		Filename:   "./logs/rss-zero.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     30,
		Compress:   true,
	}
	defer lumberjacklogger.Close()

	var core zapcore.Core
	var logger *zap.Logger
	if config.C.Settings.Debug {
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
		core = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)),
			zap.NewAtomicLevelAt(zap.DebugLevel),
		)
		logger = zap.New(core, zap.AddCaller(), zap.Development(), zap.AddStacktrace(zap.ErrorLevel))
	} else {
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
		ws := zapcore.AddSync(lumberjacklogger)
		core = zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), ws),
			zap.NewAtomicLevelAt(zap.InfoLevel),
		)
		logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
	}
	zap.ReplaceGlobals(logger)
	DefaultLogger = logger

	return logger
}
