package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type DatabaseSpan struct {
	trace.Span

	Close func(error)
}

// CreateDatabaseSpan creates a span for database operations and returns a function to close the span
func CreateDatabaseSpan(ctx context.Context, operation, table string) (context.Context, *DatabaseSpan) {
	ctx, span := StartSpan(ctx, "db."+operation)
	span.SetAttributes(
		attribute.String("db.system", "dynamodb"),
		attribute.String("db.operation", operation),
		attribute.String("db.table", table),
	)
	return ctx, &DatabaseSpan{
		Span: span,
		Close: func(err error) {
			if err != nil {
				SetError(ctx, err)
			}
			span.End()
		},
	}
}
