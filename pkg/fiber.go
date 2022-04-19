package pkg

import (
	"context"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
	otelcontrib "go.opentelemetry.io/contrib"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var (
	projectName  = GetEnvDefault("PROJECT_NAME", "fiber")
	namespace    = GetEnvDefault("NAMESPACE", "default")
	serviceName  = projectName + "." + namespace
	tracerKey    = "gofiber-contrib-tracer-fiber"
	ExtraHeaders = []string{
		// All applications should propagate x-request-id. This header is
		// included in access log statements and is used for consistent trace
		// sampling and log sampling decisions in Istio.
		"x-request-id",
		// Lightstep tracing header. Propagate this if you use lightstep tracing
		// in Istio (see
		// https://istio.io/latest/docs/tasks/observability/distributed-tracing/lightstep/)
		// Note: this should probably be changed to use B3 or W3C TRACE_CONTEXT.
		// Lightstep recommends using B3 or TRACE_CONTEXT and most application
		// libraries from lightstep do not support x-ot-span-context.
		"x-ot-span-context",
	}
)

// Middleware returns fiber handler which will trace incoming requests.
func Middleware(service string, opts ...Option) fiber.Handler {
	cfg := config{}
	for _, opt := range opts {
		opt.apply(&cfg)
	}

	if cfg.TracerProvider == nil {
		cfg.TracerProvider = otel.GetTracerProvider()
	}

	tracer := cfg.TracerProvider.Tracer(
		serviceName,
		oteltrace.WithInstrumentationVersion(otelcontrib.SemVersion()),
	)

	if cfg.Propagators == nil {
		cfg.Propagators = otel.GetTextMapPropagator()
	}

	if cfg.SpanNameFormatter == nil {
		cfg.SpanNameFormatter = defaultSpanNameFormatter
	}

	return func(c *fiber.Ctx) error {
		c.Locals(tracerKey, tracer)
		savedCtx, cancel := context.WithCancel(c.Context())

		defer func() {
			c.SetUserContext(savedCtx)
			cancel()
		}()

		reqHeader := make(http.Header)
		c.Request().Header.VisitAll(func(key, value []byte) {
			reqHeader.Add(string(key), string(value))
		})

		ctx := cfg.Propagators.Extract(savedCtx, propagation.HeaderCarrier(reqHeader))
		opts := []oteltrace.SpanStartOption{
			oteltrace.WithAttributes(
				semconv.HTTPServerNameKey.String(service),
				semconv.HTTPMethodKey.String(c.Method()),
				semconv.HTTPTargetKey.String(string(c.Request().RequestURI())),
				semconv.HTTPURLKey.String(c.OriginalURL()),
				semconv.NetHostIPKey.String(c.IP()),
				semconv.NetHostNameKey.String(c.Hostname()),
				semconv.HTTPUserAgentKey.String(string(c.Request().Header.UserAgent())),
				semconv.HTTPRequestContentLengthKey.Int(c.Request().Header.ContentLength()),
				semconv.HTTPSchemeKey.String(c.Protocol()),
				semconv.NetTransportTCP),
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		}
		if len(c.IPs()) > 0 {
			opts = append(opts, oteltrace.WithAttributes(semconv.HTTPClientIPKey.String(c.IPs()[0])))
		}
		// temporary set to c.Path() first
		// update with c.Route().Path after c.Next() is called
		// to get pathRaw
		spanName := c.Path()
		ctx, span := tracer.Start(ctx, spanName, opts...)

		// add istio envoy request id or set extra request headers
		for i := range ExtraHeaders {
			if reqHeader.Get(ExtraHeaders[i]) != "" {
				ctx = context.WithValue(ctx, ExtraHeaders[i], reqHeader.Get(ExtraHeaders[i]))
			}
		}
		defer span.End()

		// pass the span through userContext
		c.SetUserContext(ctx)

		// serve the request to the next middleware
		err := c.Next()

		span.SetName(cfg.SpanNameFormatter(c))
		span.SetAttributes(semconv.HTTPRouteKey.String(c.Route().Path))

		if err != nil {
			span.RecordError(err)
			_ = c.App().Config().ErrorHandler(c, err)
		}

		attrs := semconv.HTTPAttributesFromHTTPStatusCode(c.Response().StatusCode())
		spanStatus, spanMessage := semconv.SpanStatusFromHTTPStatusCodeAndSpanKind(c.Response().StatusCode(), oteltrace.SpanKindServer)
		span.SetAttributes(attrs...)
		span.SetStatus(spanStatus, spanMessage)

		return nil
	}
}

// defaultSpanNameFormatter is the default formatter for spans created with the fiber
// integration. Returns the route pathRaw
func defaultSpanNameFormatter(ctx *fiber.Ctx) string {
	return ctx.Route().Path
}

func GetEnvDefault(key, defVal string) string {
	val, exist := os.LookupEnv(key)
	if !exist {
		return defVal
	}
	return val
}

// for request
func GetOtelSpanHeaders(ctx *fiber.Ctx) http.Header {
	carrier := make(http.Header)
	b3prop := b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader))
	b3prop.Inject(ctx.UserContext(), propagation.HeaderCarrier(carrier))

	for _, k := range ExtraHeaders {
		value := ctx.UserContext().Value(k)
		if value != nil {
			carrier.Add(k, value.(string))
		}
	}
	return carrier
}

func GetCurrentContextSpan(ctx *fiber.Ctx) oteltrace.SpanContext {
	spanCtx := oteltrace.SpanContextFromContext(ctx.UserContext())
	return spanCtx
}
