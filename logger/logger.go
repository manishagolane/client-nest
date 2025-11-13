package logger

import (
	"fmt"
	"time"

	"github.com/manishagolane/client-nest/config"
	"github.com/manishagolane/client-nest/constants"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logger *zap.Logger

func Init() *zap.Logger {
	environment := config.GetString("environment")

	var err error
	if environment != "development" {
		writer := zapcore.AddSync(&lumberjack.Logger{
			Filename:   config.GetString("log.fileName"),
			MaxSize:    config.GetInt("log.maxSize"), // megabytes
			MaxBackups: config.GetInt("log.maxBackups"),
			MaxAge:     config.GetInt("log.maxAge"), // days
			Compress:   true,
		})
		cfg := zap.NewProductionEncoderConfig()
		location, _ := time.LoadLocation(constants.TIMEZONE)
		cfg.EncodeTime = func(t time.Time, pae zapcore.PrimitiveArrayEncoder) {
			t = t.In(location)
			zapcore.ISO8601TimeEncoder(t, pae)
		}
		cfg.EncodeDuration = zapcore.MillisDurationEncoder
		core := zapcore.NewTee(zapcore.NewCore(
			zapcore.NewJSONEncoder(cfg),
			writer,
			zap.DebugLevel,
		))
		logger = zap.New(core)
	} else {
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		logger, err = config.Build()
	}
	if err != nil {
		panic(fmt.Errorf("unable to initialize logger\n %w", err))
	}
	// defer logger.Sync()

	logger = logger.WithOptions(zap.AddCaller())
	return logger
}

func GetLogger() *zap.Logger {
	if logger == nil {
		fmt.Println("⚠️ Logger not initialized! Using fallback no-op logger.")
		return zap.NewNop() // Returns a no-op logger to prevent panic
	}
	return logger
}
