package ion

import "go.opentelemetry.io/otel/attribute"

// Attr is a key-value pair used for trace span attributes and metric dimensions.
// This is an alias for the OpenTelemetry [attribute.KeyValue] type.
//
// Create attributes using the standard OTel constructors:
//
//	import "go.opentelemetry.io/otel/attribute"
//
//	span.SetAttributes(
//	    attribute.String("order.id", orderID),
//	    attribute.Int64("retry.count", 3),
//	)
//
// Or for metrics:
//
//	counter.Add(ctx, 1, metric.WithAttributes(
//	    attribute.String("shard_id", "3"),
//	))
//
// Note: Ion intentionally does NOT wrap attribute constructors. This ensures
// users learn the standard OpenTelemetry API, which is an industry skill.
type Attr = attribute.KeyValue

// AttrKey is a type alias for attribute keys.
// Use attribute.Key("mykey").String("value") for advanced patterns.
type AttrKey = attribute.Key
