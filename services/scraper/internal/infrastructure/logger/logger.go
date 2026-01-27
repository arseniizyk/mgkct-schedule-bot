package logger

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func LoggingUnaryInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		start := time.Now()
		resp, err = handler(ctx, req)
		st := status.Convert(err)

		log.Info("gRPC request",
			"method", info.FullMethod,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", st.Message(),
			"code", st.Code().String(),
		)
		return resp, err
	}
}
