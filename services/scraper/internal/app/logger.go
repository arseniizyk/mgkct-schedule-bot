package app

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func loggingUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	start := time.Now()
	resp, err = handler(ctx, req)
	st := status.Convert(err)

	slog.Info("gRPC request",
		"method", info.FullMethod,
		"duration_ms", time.Since(start).Milliseconds(),
		"error", st.Message(),
		"code", st.Code().String(),
	)
	return resp, err
}
