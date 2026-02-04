package appcommon

import (
	"errors"
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
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

func TestApp_Config_Logger(t *testing.T) {
	t.Run("custom logger from config is used", func(t *testing.T) {
		defer resetTracingGlobals(t)

		customLogger := &testLogger{}
		tracer := mocktracer.New()
		app, err := New(Config{
			ServiceName:       "test",
			InstrumentBuckets: "0.1",
			Log:               customLogger,
		}, prometheus.NewRegistry(), "", tracer)
		require.NoError(t, err)
		require.Equal(t, customLogger, app.Logger)

		err = app.Close()
		require.NoError(t, err)
	})

	t.Run("default logger created if config logger is nil", func(t *testing.T) {
		defer resetTracingGlobals(t)

		tracer := mocktracer.New()
		app, err := New(Config{
			ServiceName:       "test",
			InstrumentBuckets: "0.1",
			Log:               nil,
		}, prometheus.NewRegistry(), "", tracer)
		require.NoError(t, err)
		require.NotNil(t, app.Logger)

		err = app.Close()
		require.NoError(t, err)
	})
}

type testLogger struct {
	logs [][]interface{}
}

func (l *testLogger) Log(keyvals ...interface{}) error {
	l.logs = append(l.logs, keyvals)
	return nil
}

func resetTracingGlobals(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	opentracing.SetGlobalTracer(nil)
	require.Equal(t, nil, opentracing.GlobalTracer())
}
