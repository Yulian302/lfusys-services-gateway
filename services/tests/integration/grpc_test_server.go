package integration

import (
	"context"
	"sync/atomic"
	"time"

	pb "github.com/Yulian302/lfusys-services-commons/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type testFilesServer struct {
	pb.UnimplementedUploaderServer
	mode atomic.Int32 // 0=ok, 1=error, 2=slow
}

func (s *testFilesServer) GetFiles(ctx context.Context, req *pb.UserInfo) (*pb.FilesReply, error) {
	switch s.mode.Load() {
	case 0:
		return &pb.FilesReply{}, nil
	case 1:
		return nil, status.Error(codes.Unavailable, "down")
	case 2:
		time.Sleep(200 * time.Millisecond)
		return &pb.FilesReply{}, nil
	default:
		return nil, status.Error(codes.Internal, "unknown")
	}
}
