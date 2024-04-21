package log

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/eli-yip/rss-zero/config"
)

// NewZapLogger returns a new zap logger.
// To enable debug mode, add DEBUG=true to .env file.
// It will use production config otherwise.
func NewZapLogger() *zap.Logger {
	var zapConfig zap.Config
	if config.C.Debug {
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		zapConfig = zap.NewProductionConfig()
	}
	zapConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")

	logger, err := zapConfig.Build()
	if err != nil {
		panic(fmt.Sprintf("Failed to init zap logger: %v", err))
	}

	return logger
}
