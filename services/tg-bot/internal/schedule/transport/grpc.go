package transport

import (
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"google.golang.org/grpc"
)

type GRPCStub struct {
	pb.ScheduleServiceClient
}

func New(conn *grpc.ClientConn) *GRPCStub {
	client := pb.NewScheduleServiceClient(conn)

	return &GRPCStub{client}
}
