package middleware

import (
	"context"
	"fmt"
	"net/http"

	opentracing "github.com/opentracing/opentracing-go"
	"go.opentelemetry.io/otel/trace"

	"github.com/opentracing-contrib/go-stdlib/nethttp"
	jaeger "github.com/uber/jaeger-client-go"
)

// Tracer is a middleware which traces incoming requests.
type Tracer struct {
	routeMatcher RouteMatcher
}

func NewTracer(routeMatcher RouteMatcher, tracer opentracing.Tracer) *Tracer {
	if tracer != nil {
		opentracing.SetGlobalTracer(tracer)
	}
	return &Tracer{
		routeMatcher: routeMatcher,
	}
}

// Wrap implements Interface
func (t Tracer) Wrap(next http.Handler) http.Handler {
	options := []nethttp.MWOption{
		nethttp.OperationNameFunc(func(r *http.Request) string {
			op := getRouteName(t.routeMatcher, r)
			if op == "" {
				return "HTTP " + r.Method
			}

			return fmt.Sprintf("HTTP %s - %s", r.Method, op)
		}),
	}

	return nethttp.Middleware(opentracing.GlobalTracer(), next, options...)
}

// ExtractTraceID extracts the trace id, if any from the context.
func ExtractTraceID(ctx context.Context) (string, bool) {
	traceID, _ := ExtractSampledTraceID(ctx)
	return traceID, traceID != ""
}

// ExtractSampledTraceID works like ExtractTraceID but the returned bool is only
// true if the returned trace id is sampled.
func ExtractSampledTraceID(ctx context.Context) (string, bool) {
	// the most common case, where jaeger and opentracing is used
	sp := opentracing.SpanFromContext(ctx)
	if sp != nil {
		sctx, ok := sp.Context().(jaeger.SpanContext)
		if ok {
			return sctx.TraceID().String(), sctx.IsSampled()
		}
	}

	// opentelemetry with and without the bridge
	otelSp := trace.SpanFromContext(ctx)
	traceID, sampled := otelSp.SpanContext().TraceID(), otelSp.SpanContext().IsSampled()
	if traceID.IsValid() { // when noop span is used, the traceID is not valid
		return traceID.String(), sampled
	}

	// when nothing is in the context
	return "", false
}
