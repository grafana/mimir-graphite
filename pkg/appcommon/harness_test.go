package appcommon

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/grafana/dskit/user"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/mimir-graphite/v2/pkg/server"
	"github.com/grafana/mimir-graphite/v2/pkg/server/middleware"
)

func TestApp_Close(t *testing.T) {
	t.Run("single error", func(t *testing.T) {
		app := App{closers: []func() error{
			func() error { return errors.New("yikes") },
		}}

		err := app.Close()
		require.Error(t, err)
		require.Equal(t, "error 1: yikes", err.Error())
	})

	t.Run("multiple errors", func(t *testing.T) {
		app := App{closers: []func() error{
			func() error { return errors.New("yikes") },
			func() error { return errors.New("arghhhhh") },
		}}

		err := app.Close()
		require.Equal(t, "error 1: yikes, error 2: arghhhhh", err.Error())
	})

	t.Run("single failed close with multiple successful closes errors", func(t *testing.T) {
		app := App{closers: []func() error{
			func() error { return nil },
			func() error { return errors.New("arghhhhh") },
			func() error { return nil },
		}}

		err := app.Close()
		require.Error(t, err)
		require.Equal(t, "error 1: arghhhhh", err.Error())
	})

	t.Run("no closers no errors", func(t *testing.T) {
		app := App{}

		err := app.Close()
		require.NoError(t, err)
	})

	t.Run("empty closers no errors", func(t *testing.T) {
		app := App{closers: []func() error{}}
		err := app.Close()
		require.NoError(t, err)
	})

	t.Run("all closers successful no errors", func(t *testing.T) {
		app := App{closers: []func() error{
			func() error { return nil },
			func() error { return nil },
		}}

		err := app.Close()
		require.NoError(t, err)
	})

}

func TestApp_Config_Tracer(t *testing.T) {
	t.Run("tracer from config is set as global tracer", func(t *testing.T) {
		defer resetTracingGlobals(t)

		tracer := mocktracer.New()
		app, err := New(Config{ServiceName: "test", InstrumentBuckets: "0.1"}, prometheus.NewRegistry(), "", tracer)
		require.NoError(t, err)
		require.Equal(t, tracer, opentracing.GlobalTracer())

		err = app.Close()
		require.NoError(t, err)
	})

	t.Run("new global tracer created if config tracer is nil", func(t *testing.T) {
		defer resetTracingGlobals(t)

		app, err := New(Config{ServiceName: "test", InstrumentBuckets: "0.1"}, prometheus.NewRegistry(), "", nil)
		require.NoError(t, err)
		require.NotNil(t, opentracing.GlobalTracer())

		err = app.Close()
		require.NoError(t, err)
	})
}

func TestApp_CustomAuthMiddleware(t *testing.T) {
	t.Run("injects orgID into context", func(t *testing.T) {
		defer resetTracingGlobals(t)

		customAuth := middleware.Func(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := user.InjectOrgID(r.Context(), "custom-org")
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})

		app, err := New(Config{
			ServiceName:       "test",
			InstrumentBuckets: "0.1",
			AuthMiddleware:    customAuth,
			ServerConfig:      serverConfigWithPort0(),
		}, prometheus.NewRegistry(), "", mocktracer.New())
		require.NoError(t, err)
		defer func() { require.NoError(t, app.Close()) }()

		go func() { _ = app.Server.Run() }()

		app.Server.Router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			orgID, err := user.ExtractOrgID(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_, _ = fmt.Fprint(w, orgID)
		})

		resp, err := http.Get(fmt.Sprintf("http://%s/test", app.Server.Addr()))
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "custom-org", string(body))
	})

	t.Run("rejects request with 401", func(t *testing.T) {
		defer resetTracingGlobals(t)

		customAuth := middleware.Func(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			})
		})

		app, err := New(Config{
			ServiceName:       "test",
			InstrumentBuckets: "0.1",
			AuthMiddleware:    customAuth,
			ServerConfig:      serverConfigWithPort0(),
		}, prometheus.NewRegistry(), "", mocktracer.New())
		require.NoError(t, err)
		defer func() { require.NoError(t, app.Close()) }()

		go func() { _ = app.Server.Run() }()

		app.Server.Router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, "should not reach here")
		})

		resp, err := http.Get(fmt.Sprintf("http://%s/test", app.Server.Addr()))
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		require.Contains(t, string(body), "unauthorized")
	})
}

func serverConfigWithPort0() server.Config {
	return server.Config{
		HTTPListenPort: 0,
		GRPCListenPort: 0,
	}
}

func resetTracingGlobals(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	opentracing.SetGlobalTracer(nil)
	require.Equal(t, nil, opentracing.GlobalTracer())
}
