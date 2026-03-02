package appcommon

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/mwitkow/go-conntrack"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/gorilla/mux"
	"github.com/oklog/run"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/mimir-graphite/v2/pkg/ctxlog"
	"github.com/grafana/mimir-graphite/v2/pkg/internalserver"
	"github.com/grafana/mimir-graphite/v2/pkg/server"
	"github.com/grafana/mimir-graphite/v2/pkg/server/middleware"
	"github.com/grafana/mimir-graphite/v2/pkg/stopsignal"
)

var (
	CommitUnixTimestamp = "0"
	DockerTag           = "unset"
)

type Config struct {
	InstrumentBuckets string `yaml:"instrument_buckets"`
	EnableAuth        bool   `yaml:"enable_auth"`
	ServiceName       string `yaml:"service_name"`

	// AuthMiddleware, if set, replaces the default auth middleware
	// (HTTPAuth or HTTPFakeAuth). This allows consumers to provide
	// custom authentication that runs before the logging middleware.
	AuthMiddleware middleware.Interface `yaml:"-"`

	ServerConfig         server.Config         `yaml:"server_config"`
	InternalServerConfig internalserver.Config `yaml:"internal_server_config"`
}

// RegisterFlags adds the flags required to config this to the given FlagSet
func (cfg *Config) RegisterFlags(flags *flag.FlagSet) {
	cfg.RegisterFlagsWithPrefix("", flags)
}

// RegisterFlagsWithPrefix registers flags, adding the provided prefix if
// needed. If the prefix is not blank and doesn't end with '.', a '.' is
// appended to it.
func (cfg *Config) RegisterFlagsWithPrefix(prefix string, flags *flag.FlagSet) {
	if prefix != "" && !strings.HasSuffix(prefix, ".") {
		prefix += "."
	}
	flags.StringVar(&cfg.InstrumentBuckets, prefix+"instrument-buckets", ".005,.010,.015,.020,.025,.050,.100,.250,.500,1,2.5,5,10", "Buckets for instrumentation, comma separated list of seconds as floats.")
	flags.BoolVar(&cfg.EnableAuth, prefix+"auth.enable", true, "require X-Scope-OrgId header")
	flags.StringVar(&cfg.ServiceName, prefix+"service-name", "", "the service name used in traces")

	cfg.ServerConfig.RegisterFlagsWithPrefix(prefix, flags)
	cfg.InternalServerConfig.RegisterFlagsWithPrefix(prefix, flags)
}

type App struct {
	Group *run.Group

	Logger      log.Logger
	LogProvider ctxlog.Provider
	Server      *server.Server
	Tracer      opentracing.Tracer
	closers     []func() error
}

func init() {
	// Monitor outgoing connections on default transport with conntrack.
	http.DefaultTransport.(*http.Transport).DialContext = conntrack.NewDialContextFunc(
		conntrack.DialWithName("unknown"),
		conntrack.DialWithTracing(),
	)
}

// New creates a new App.
// Callers should call App.Close() after use.
func New(cfg Config, reg prometheus.Registerer, metricPrefix string, tracer opentracing.Tracer) (app App, err error) {
	if cfg.ServiceName == "" {
		return app, fmt.Errorf("service name can't be empty")
	}

	app = App{
		Group: &run.Group{},
	}
	// If the function fails, make sure all resources are cleaned up before returning the error.
	defer func() {
		if err != nil {
			_ = app.Close()
		}
	}()

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	logger = log.WithPrefix(logger, "ts", log.DefaultTimestampUTC)
	app.Logger = logger
	app.LogProvider = ctxlog.NewProvider(logger)

	router := mux.NewRouter()

	// Configure middlewares
	defBuckets, err := parseFloats(cfg.InstrumentBuckets)
	if err != nil {
		return app, fmt.Errorf("can't parse instrument buckets: %w", err)
	}
	instrumentMiddleware, err := middleware.NewInstrument(router, defBuckets, metricPrefix)
	if err != nil {
		return app, fmt.Errorf("can't initialize the instrumentation middleware %w", err)
	}

	if tracer != nil {
		app.Tracer = tracer
	} else {
		tracer, closer, err := NewTracer(cfg.ServiceName, logger)
		if err != nil {
			return app, err
		}
		app.closers = append(app.closers, closer.Close)
		app.Tracer = tracer
	}
	tracerMiddleware := middleware.NewTracer(router, app.Tracer)

	logMiddleware := middleware.NewLoggingMiddleware(logger)

	var authMiddleware middleware.Interface
	if cfg.AuthMiddleware != nil {
		authMiddleware = cfg.AuthMiddleware
	} else if cfg.EnableAuth {
		authMiddleware = middleware.NewHTTPAuth(logger)
	} else {
		authMiddleware = middleware.HTTPFakeAuth{}
	}

	// Middlewares will be wrapped in order
	middlewares := []middleware.Interface{
		tracerMiddleware,
		instrumentMiddleware,
		authMiddleware,
		logMiddleware,
	}

	if cfg.ServerConfig.HTTPMaxRequestSizeLimit > 0 {
		requestLimitsMiddleware := middleware.NewRequestLimitsMiddleware(cfg.ServerConfig.HTTPMaxRequestSizeLimit, logger)
		middlewares = append(middlewares, requestLimitsMiddleware)
	}

	srv, err := server.NewServer(logger, cfg.ServerConfig, router, middlewares)
	if err != nil {
		level.Error(logger).Log("msg", "failed to start server", "err", err)
		return app, fmt.Errorf("failed to start server: %w", err)
	}
	app.Server = srv

	signalHandler := stopsignal.NewSignalHandler(cfg.InternalServerConfig.ServerGracefulShutdownTimeout, logger)
	cfg.InternalServerConfig.ReadinessProvider = signalHandler

	app.Group.Add(app.Server.Handler())
	app.Group.Add(internalserver.Handler(logger, cfg.InternalServerConfig))
	app.Group.Add(signalHandler.Handler(syscall.SIGTERM, syscall.SIGINT))

	if err := registerVersionMetrics(reg, cfg.ServiceName, metricPrefix); err != nil {
		return app, err
	}
	level.Info(logger).Log("msg", "Starting app", "docker_tag", DockerTag)

	return app, nil
}

type AppError []error

func (ae AppError) Error() string {
	var sb strings.Builder
	for i, err := range ae {
		sb.WriteString(fmt.Sprintf("error %d: ", i+1))
		sb.WriteString(err.Error())
		if i < len(ae)-1 {
			sb.WriteString(", ")
		}
	}
	return sb.String()
}

func (app App) Close() error {
	var errs AppError
	for _, closerFunc := range app.closers {
		if err := closerFunc(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errs
	}
	return nil
}

func registerVersionMetrics(reg prometheus.Registerer, serviceName, metricPrefix string) error {
	buildDateGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricPrefix,
		Name:      "build_unix_timestamp",
		Help:      "A constant build date value reported by each instance as a Unix epoch timestamp.",
		ConstLabels: prometheus.Labels{
			"service_name": serviceName,
			"docker_tag":   DockerTag,
		},
	})
	if err := reg.Register(buildDateGauge); err != nil {
		return err
	}
	parsedCommitTimestamp, err := strconv.ParseFloat(CommitUnixTimestamp, 64) //nolint:gomnd
	if err != nil {
		return fmt.Errorf("can't parse CommitUnixTimestamp: %w", err)
	}
	buildDateGauge.Set(parsedCommitTimestamp)
	return nil
}

func parseFloats(str string) ([]float64, error) {
	if str == "" {
		return nil, errors.New("empty string")
	}
	strs := strings.Split(str, ",")
	vals := make([]float64, len(strs))
	var err error
	for i, s := range strs {
		vals[i], err = strconv.ParseFloat(s, 64) //nolint:gomnd
		if err != nil {
			return nil, fmt.Errorf("can't parse value %d: %w", i, err)
		}
	}
	return vals, nil
}
