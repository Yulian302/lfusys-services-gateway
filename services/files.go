package services

import (
	"context"
	"fmt"
	"time"

	pb "github.com/Yulian302/lfusys-services-commons/api"
	"github.com/Yulian302/lfusys-services-gateway/files/types"
	"github.com/sony/gobreaker/v2"
)

type FileService interface {
	GetFiles(ctx context.Context, email string) (*types.FilesResponse, error)
}

type FileServiceImpl struct {
	clientStub pb.UploaderClient
	breaker    *gobreaker.CircuitBreaker[*pb.FilesReply]
}

func NewFileServiceImpl(stub pb.UploaderClient, breaker *gobreaker.CircuitBreaker[*pb.FilesReply]) *FileServiceImpl {
	return &FileServiceImpl{
		clientStub: stub,
		breaker:    breaker,
	}
}

func (svc *FileServiceImpl) GetFiles(ctx context.Context, email string) (*types.FilesResponse, error) {

	reply, err := svc.breaker.Execute(func() (*pb.FilesReply, error) {
		grpcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		return svc.clientStub.GetFiles(grpcCtx, &pb.UserInfo{
			Email: email,
		})
	})

	if err != nil {
		return nil, fmt.Errorf("get files via grpc: %w", err)
	}

	files := make([]*types.File, len(reply.Files))
	for i, f := range reply.Files {
		files[i] = &types.File{
			FileId:      f.Id,
			UploadId:    f.UploadId,
			OwnerEmail:  f.OwnerEmail,
			Size:        f.Size,
			TotalChunks: f.TotalChunks,
			Checksum:    f.Checksum,
			CreatedAt:   f.CreatedAt.AsTime(),
		}
	}

	return &types.FilesResponse{
		Files: files,
	}, nil

}
