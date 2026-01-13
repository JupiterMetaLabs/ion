package core

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/JupiterMetaLabs/ion/internal/config"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapFactoryResult holds the result of constructing the zap logger.
type ZapFactoryResult struct {
	Logger       *zap.Logger
	AtomicLevel  zap.AtomicLevel
	OTELProvider *LogProvider
}

// NewZapLogger creates a new configured Zap logger.
// It sets up console, file, and OTEL cores as configured.
func NewZapLogger(cfg config.Config) (*ZapFactoryResult, error) {
	var otelProvider *LogProvider
	var otelCore zapcore.Core
	var err error

	atomicLevel := zap.NewAtomicLevelAt(parseLevel(cfg.Level))

	// 1. Setup OTEL if enabled
	if cfg.OTEL.Enabled && cfg.OTEL.Endpoint != "" {
		// Handle Basic Auth - inject Authorization header
		headers := cfg.OTEL.Headers
		if headers == nil {
			headers = make(map[string]string)
		}
		if cfg.OTEL.Username != "" && cfg.OTEL.Password != "" {
			auth := fmt.Sprintf("%s:%s", cfg.OTEL.Username, cfg.OTEL.Password)
			encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))

			// Use lowercase "authorization" for gRPC to comply with HTTP/2 and gRPC metadata specs.
			key := "Authorization"
			if cfg.OTEL.Protocol != "http" {
				key = "authorization"
			}
			headers[key] = "Basic " + encodedAuth
		}
		cfg.OTEL.Headers = headers // Update in place for setup

		otelProvider, err = SetupLogProvider(cfg.OTEL, cfg.ServiceName, cfg.Version)
		if err != nil {
			return nil, fmt.Errorf("otel setup failed: %w", err)
		}

		if otelProvider != nil && otelProvider.LoggerProvider() != nil {
			otelCore = otelzap.NewCore(
				cfg.ServiceName,
				otelzap.WithLoggerProvider(otelProvider.LoggerProvider()),
			)
		}
	}

	// 2. Build Cores
	cores := make([]zapcore.Core, 0, 4)

	// Console
	if cfg.Console.Enabled {
		consoleCores := buildConsoleCores(cfg, atomicLevel)
		for _, c := range consoleCores {
			cores = append(cores, NewFilteringCore(c, SentinelKey))
		}
	}

	// File
	if cfg.File.Enabled && cfg.File.Path != "" {
		fileCore := buildFileCore(cfg, atomicLevel)
		if fileCore != nil {
			cores = append(cores, NewFilteringCore(fileCore, SentinelKey))
		}
	}

	// OTEL
	if otelCore != nil {
		// Wrap with level enforcer to ensure configured level (e.g. Debug) is respected
		// even if otelzap defaults to Info.
		otelCore = &levelEnforcer{Core: otelCore, level: atomicLevel}

		// Filter trace_id/span_id strings (redundant) but keep SentinelKey
		cores = append(cores, NewFilteringCore(otelCore, "trace_id", "span_id"))
	}

	// 3. Combine
	var core zapcore.Core
	switch len(cores) {
	case 0:
		core = zapcore.NewNopCore()
	case 1:
		core = cores[0]
	default:
		core = zapcore.NewTee(cores...)
	}

	// 4. Build options
	opts := buildZapOptions(cfg)

	// Add Fatal hook to prevent exit on Critical/Fatal logs
	// This ensures Critical() logs as Fatal level but doesn't kill the process.
	opts = append(opts, zap.WithFatalHook(noExitHook{}))

	logger := zap.New(core, opts...)

	return &ZapFactoryResult{
		Logger:       logger,
		AtomicLevel:  atomicLevel,
		OTELProvider: otelProvider,
	}, nil
}

type noExitHook struct{}

func (noExitHook) OnWrite(ce *zapcore.CheckedEntry, fields []zapcore.Field) {
	// Do nothing, preventing os.Exit
}

func buildZapOptions(cfg config.Config) []zap.Option {
	opts := []zap.Option{
		zap.AddCallerSkip(1), // Skip the wrapper methods
	}

	if cfg.Development {
		opts = append(opts, zap.Development())
		opts = append(opts, zap.AddCaller())
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	if cfg.ServiceName != "" {
		opts = append(opts, zap.Fields(zap.String("service", cfg.ServiceName)))
	}
	if cfg.Version != "" {
		opts = append(opts, zap.Fields(zap.String("version", cfg.Version)))
	}

	return opts
}

func buildConsoleCores(cfg config.Config, level zap.AtomicLevel) []zapcore.Core {
	encoder := buildConsoleEncoder(cfg)

	if cfg.Console.ErrorsToStderr {
		// stdout: [configLevel, Warn)
		stdoutLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= level.Level() && lvl < zapcore.WarnLevel
		})

		// stderr: [Warn, Fatal] AND >= configLevel
		// e.g., if config=Error, stderr only shows Error+ (Warn is suppressed).
		// if config=Debug, stderr shows Warn+.
		stderrLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			min := level.Level()
			if min > zapcore.WarnLevel {
				return lvl >= min
			}
			return lvl >= zapcore.WarnLevel
		})

		return []zapcore.Core{
			zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), stdoutLevel),
			zapcore.NewCore(encoder, zapcore.Lock(os.Stderr), stderrLevel),
		}
	}

	return []zapcore.Core{
		zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), level),
	}
}

func buildConsoleEncoder(cfg config.Config) zapcore.Encoder {
	if cfg.Console.Format == "pretty" || (cfg.Development && cfg.Console.Format == "") {
		encoderCfg := zap.NewDevelopmentEncoderConfig()
		if cfg.Console.Color {
			encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
			encoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05.000")
		} else {
			encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder
			encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		}
		encoderCfg.EncodeCaller = zapcore.ShortCallerEncoder
		return zapcore.NewConsoleEncoder(encoderCfg)
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.MessageKey = "msg"
	encoderCfg.LevelKey = "level"
	encoderCfg.CallerKey = "caller"
	return zapcore.NewJSONEncoder(encoderCfg)
}

func buildFileCore(cfg config.Config, level zap.AtomicLevel) zapcore.Core {
	writer := config.NewFileWriter(cfg.File)
	if writer == nil {
		return nil
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(encoderCfg)

	return zapcore.NewCore(encoder, zapcore.AddSync(writer), level)
}

func parseLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}
