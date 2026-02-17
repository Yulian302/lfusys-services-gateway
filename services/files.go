package services

import (
	"context"
	"fmt"
	"time"

	pb "github.com/Yulian302/lfusys-services-commons/api/uploader/v1"
	"github.com/Yulian302/lfusys-services-gateway/files/types"
	"github.com/sony/gobreaker/v2"
)

type FileService interface {
	GetFiles(ctx context.Context, email string) (*types.FilesResponse, error)
	Delete(ctx context.Context, fileId string) error
}

type FileServiceImpl struct {
	clientStub pb.UploaderClient
	breaker    *gobreaker.CircuitBreaker[any]
}

func NewFileServiceImpl(stub pb.UploaderClient, breaker *gobreaker.CircuitBreaker[any]) *FileServiceImpl {
	return &FileServiceImpl{
		clientStub: stub,
		breaker:    breaker,
	}
}

func (svc *FileServiceImpl) GetFiles(ctx context.Context, email string) (*types.FilesResponse, error) {
	replyRaw, err := svc.breaker.Execute(func() (any, error) {
		grpcCtx, cancel := context.WithDeadline(ctx, time.Now().Add(2*time.Second))
		defer cancel()

		return svc.clientStub.GetFiles(grpcCtx, &pb.UserInfo{
			Email: email,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("get files via grpc: %w", err)
	}

	reply, ok := replyRaw.(*pb.FilesReply)
	if !ok {
		return nil, fmt.Errorf("unexpected response type from breaker")
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

func (svc *FileServiceImpl) Delete(ctx context.Context, fileId string) error {
	_, err := svc.breaker.Execute(func() (any, error) {
		grpcCtx, cancel := context.WithDeadline(ctx, time.Now().Add(2*time.Second))
		defer cancel()

		return svc.clientStub.DeleteFile(grpcCtx, &pb.FileDeleteRequest{
			FileId: fileId,
		})
	})
	if err != nil {
		return fmt.Errorf("delete file via grpc: %w", err)
	}

	return nil
}
