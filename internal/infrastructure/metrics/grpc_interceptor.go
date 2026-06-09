package metrics

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryClientInterceptor returns a gRPC client interceptor that collects Prometheus metrics.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		start := time.Now()

		err := invoker(ctx, method, req, reply, cc, opts...)

		duration := time.Since(start).Milliseconds()
		statusCode := status.Code(err).String()

		GRPCRequestDuration.WithLabelValues(method, statusCode).Observe(float64(duration))
		GRPCRequestTotal.WithLabelValues(method, statusCode).Inc()

		return err
	}
}
