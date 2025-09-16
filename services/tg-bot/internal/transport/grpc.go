package transport

import (
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"google.golang.org/grpc"
)

func New(conn *grpc.ClientConn) pb.ScheduleServiceClient {
	return pb.NewScheduleServiceClient(conn)
}
