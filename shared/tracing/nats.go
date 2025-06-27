package tracing

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// InjectNATSHeaders injects trace context into NATS message headers
func InjectNATSHeaders(ctx context.Context, msg *nats.Msg) {
	if msg.Header == nil {
		msg.Header = make(nats.Header)
	}
	GetPropagator().Inject(ctx, &natsHeaderCarrier{msg.Header})
}

// ExtractNATSHeaders extracts trace context from NATS message headers
func ExtractNATSHeaders(ctx context.Context, msg *nats.Msg) context.Context {
	if msg.Header == nil {
		return ctx
	}
	return GetPropagator().Extract(ctx, &natsHeaderCarrier{msg.Header})
}

// CreateNATSPublishSpan creates a span for NATS message publishing
func CreateNATSPublishSpan(ctx context.Context, subject string) (context.Context, trace.Span) {
	ctx, span := StartSpan(ctx, "messagebus.publish "+subject)
	span.SetAttributes(
		attribute.String("messagebus.system", "nats"),
		attribute.String("messagebus.operation", "publish"),
		attribute.String("messagebus.destination", subject),
	)
	return ctx, span
}

// CreateNATSConsumeSpan creates a span for NATS message consumption
func CreateNATSConsumeSpan(ctx context.Context, subject string) (context.Context, trace.Span) {
	ctx, span := StartSpan(ctx, "messagebus.consume "+subject)
	span.SetAttributes(
		attribute.String("messagebus.system", "nats"),
		attribute.String("messagebus.operation", "consume"),
		attribute.String("messagebus.source", subject),
	)
	return ctx, span
}

// natsHeaderCarrier implements TextMapCarrier for NATS headers
type natsHeaderCarrier struct {
	header nats.Header
}

func (n *natsHeaderCarrier) Get(key string) string {
	return n.header.Get(key)
}

func (n *natsHeaderCarrier) Set(key, value string) {
	n.header.Set(key, value)
}

func (n *natsHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(n.header))
	for k := range n.header {
		keys = append(keys, k)
	}
	return keys
}
