package pkg

const (
	// All applications should propagate x-request-id. This header is
	// included in access log statements and is used for consistent trace
	// sampling and log sampling decisions in Istio.
	RequestId = "x-request-id"
	// Lightstep tracing header. Propagate this if you use lightstep tracing
	// in Istio (see
	// https://istio.io/latest/docs/tasks/observability/distributed-tracing/lightstep/)
	// Note: this should probably be changed to use B3 or W3C TRACE_CONTEXT.
	// Lightstep recommends using B3 or TRACE_CONTEXT and most application
	// libraries from lightstep do not support x-ot-span-context.
	OtSpanContext = "x-ot-span-context"
	// Datadog tracing header. Propagate these headers if you use Datadog
	// tracing.
	DataDogTraceId          = "x-datadog-trace-id"
	DataDogParentId         = "x-datadog-parent-id"
	DataDogSamplingPriority = "x-datadog-sampling-priority"
)
