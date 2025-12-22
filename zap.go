package ion

import (
	"context"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// zapLogger implements Logger using Uber's Zap.
type zapLogger struct {
	zap       *zap.Logger
	config    Config
	atomicLvl zap.AtomicLevel
}

// New creates a new Logger from the provided configuration.
// This is the main constructor for ion.
func New(cfg Config) Logger {
	return buildLogger(cfg, nil)
}

// NewWithOTEL creates a logger with OTEL export enabled.
// This wires Zap logs to the OpenTelemetry log pipeline.
func NewWithOTEL(cfg Config) (Logger, error) {
	// First set up OTEL provider
	provider, err := SetupOTEL(cfg.OTEL, cfg.ServiceName, cfg.Version)
	if err != nil {
		return nil, err
	}

	var otelCore zapcore.Core
	if provider != nil && provider.provider != nil {
		otelCore = otelzap.NewCore(
			cfg.ServiceName,
			otelzap.WithLoggerProvider(provider.provider),
		)
	}

	return buildLogger(cfg, otelCore), nil
}

// buildLogger constructs the zapLogger with all configured cores.
// If otelCore is non-nil, it's added to the core tee.
func buildLogger(cfg Config, otelCore zapcore.Core) Logger {
	atomicLevel := zap.NewAtomicLevelAt(parseLevel(cfg.Level))
	cores := make([]zapcore.Core, 0, 4)

	// Console output
	if cfg.Console.Enabled {
		consoleCores := buildConsoleCores(cfg, atomicLevel)
		cores = append(cores, consoleCores...)
	}

	// File output (with rotation)
	if cfg.File.Enabled && cfg.File.Path != "" {
		fileCore := buildFileCore(cfg, atomicLevel)
		if fileCore != nil {
			cores = append(cores, fileCore)
		}
	}

	// OTEL core (if provided)
	if otelCore != nil {
		cores = append(cores, otelCore)
	}

	// Combine all cores
	var core zapcore.Core
	switch len(cores) {
	case 0:
		core = zapcore.NewNopCore()
	case 1:
		core = cores[0]
	default:
		core = zapcore.NewTee(cores...)
	}

	// Build options
	opts := buildZapOptions(cfg)
	logger := zap.New(core, opts...)

	return &zapLogger{
		zap:       logger,
		config:    cfg,
		atomicLvl: atomicLevel,
	}
}

// buildZapOptions creates common zap options from config.
func buildZapOptions(cfg Config) []zap.Option {
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

// buildConsoleCores creates console output cores.
// Returns multiple cores if ErrorsToStderr is enabled (stdout for info, stderr for errors).
func buildConsoleCores(cfg Config, level zap.AtomicLevel) []zapcore.Core {
	encoder := buildConsoleEncoder(cfg)

	if cfg.Console.ErrorsToStderr {
		// Split: debug/info → stdout, warn/error/fatal → stderr
		stdoutLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= level.Level() && lvl < zapcore.WarnLevel
		})
		stderrLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
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

// buildConsoleEncoder creates the appropriate encoder for console output.
func buildConsoleEncoder(cfg Config) zapcore.Encoder {
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

	// JSON encoder for production
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.MessageKey = "msg"
	encoderCfg.LevelKey = "level"
	encoderCfg.CallerKey = "caller"
	return zapcore.NewJSONEncoder(encoderCfg)
}

// buildFileCore creates the file output core with rotation.
func buildFileCore(cfg Config, level zap.AtomicLevel) zapcore.Core {
	writer := NewFileWriter(cfg.File)
	if writer == nil {
		return nil
	}

	// Always use JSON for file output
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(encoderCfg)

	return zapcore.NewCore(encoder, zapcore.AddSync(writer), level)
}

// parseLevel converts a string level to zapcore.Level.
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

// --- Logger interface implementation ---

func (l *zapLogger) Debug(msg string, fields ...Field) {
	l.zap.Debug(msg, toZapFields(fields)...)
}

func (l *zapLogger) Info(msg string, fields ...Field) {
	l.zap.Info(msg, toZapFields(fields)...)
}

func (l *zapLogger) Warn(msg string, fields ...Field) {
	l.zap.Warn(msg, toZapFields(fields)...)
}

func (l *zapLogger) Error(msg string, err error, fields ...Field) {
	zapFields := toZapFields(fields)
	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}
	l.zap.Error(msg, zapFields...)
}

func (l *zapLogger) Fatal(msg string, err error, fields ...Field) {
	zapFields := toZapFields(fields)
	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}
	l.zap.Fatal(msg, zapFields...)
}

func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{
		zap:       l.zap.With(toZapFields(fields)...),
		config:    l.config,
		atomicLvl: l.atomicLvl,
	}
}

func (l *zapLogger) WithContext(ctx context.Context) Logger {
	contextFields := extractContextFields(ctx)
	if len(contextFields) == 0 {
		return l
	}
	return l.With(contextFields...)
}

func (l *zapLogger) Named(name string) Logger {
	return &zapLogger{
		zap:       l.zap.Named(name),
		config:    l.config,
		atomicLvl: l.atomicLvl,
	}
}

func (l *zapLogger) Sync() error {
	return l.zap.Sync()
}

// SetLevel changes the log level at runtime.
// This is safe to call from multiple goroutines.
func (l *zapLogger) SetLevel(level string) {
	l.atomicLvl.SetLevel(parseLevel(level))
}

// GetLevel returns the current log level.
func (l *zapLogger) GetLevel() string {
	return l.atomicLvl.Level().String()
}

// --- Field conversion ---

// toZapFields converts ion.Field slice to zap.Field slice.
func toZapFields(fields []Field) []zap.Field {
	if len(fields) == 0 {
		return nil
	}

	zapFields := make([]zap.Field, 0, len(fields))
	for _, f := range fields {
		zapFields = append(zapFields, zap.Any(f.Key, f.Value))
	}
	return zapFields
}
