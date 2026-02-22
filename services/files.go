package services

import (
	"context"
	"fmt"
	"time"

	pb "github.com/Yulian302/lfusys-services-commons/api/uploader/v1"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/Yulian302/lfusys-services-gateway/files/types"
	"github.com/sony/gobreaker/v2"
	"google.golang.org/protobuf/types/known/durationpb"
)

type FileService interface {
	GetFiles(ctx context.Context, email string) (*types.FilesResponse, error)
	GetByID(ctx context.Context, fileId, email string) (*types.File, error)
	GetDownloadURL(ctx context.Context, fileId string, ttl time.Duration) (string, error)
	Delete(ctx context.Context, fileId, ownerEmail string) error
}

type FileServiceImpl struct {
	clientStub pb.UploaderClient
	breaker    *gobreaker.CircuitBreaker[any]

	logger logger.Logger
}

func NewFileServiceImpl(stub pb.UploaderClient, breaker *gobreaker.CircuitBreaker[any], l logger.Logger) *FileServiceImpl {
	return &FileServiceImpl{
		clientStub: stub,
		breaker:    breaker,
		logger:     l,
	}
}

func (svc *FileServiceImpl) GetFiles(ctx context.Context, email string) (*types.FilesResponse, error) {
	replyRaw, err := svc.breaker.Execute(func() (any, error) {
		grpcCtx, cancel := context.WithDeadline(ctx, time.Now().Add(2*time.Second))
		defer cancel()

		svc.logger.Info("circuit breaker state is ", svc.breaker.State())

		return svc.clientStub.GetFiles(grpcCtx, &pb.UserInfo{
			Email: email,
		})
	})
	if err != nil {
		svc.logger.Error("grpc GetFiles failed",
			"email", email,
			"error", err,
		)
		return nil, fmt.Errorf("get files via grpc: %w", err)
	}

	reply, ok := replyRaw.(*pb.FilesReply)
	if !ok {
		svc.logger.Error("unexpected breaker response type")
		return nil, fmt.Errorf("unexpected response type from breaker")
	}

	files := make([]*types.File, len(reply.Files))
	for i, f := range reply.Files {
		files[i] = &types.File{
			FileId:      f.Id,
			UploadId:    f.UploadId,
			OwnerEmail:  f.OwnerEmail,
			Name:        f.Name,
			Type:        f.Type,
			Size:        f.Size,
			TotalChunks: f.TotalChunks,
			Checksum:    f.Checksum,
			CreatedAt:   f.CreatedAt.AsTime(),
		}
	}

	svc.logger.Debug("grpc GetFiles succeeded",
		"email", email,
		"file_count", len(reply.Files),
	)

	return &types.FilesResponse{
		Files: files,
	}, nil
}

func (svc *FileServiceImpl) GetByID(ctx context.Context, fileId, email string) (*types.File, error) {
	replyRaw, err := svc.breaker.Execute(func() (any, error) {
		grpcCtx, cancel := context.WithDeadline(ctx, time.Now().Add(2*time.Second))
		defer cancel()

		svc.logger.Info("circuit breaker state is ", svc.breaker.State())

		return svc.clientStub.GetFileById(grpcCtx, &pb.FileByIdRequest{
			Email:  email,
			FileId: fileId,
		})
	})
	if err != nil {
		svc.logger.Error("grpc GetByID failed",
			"email", email,
			"error", err,
		)
		return nil, fmt.Errorf("get file by id via grpc: %w", err)
	}

	file, ok := replyRaw.(*pb.File)
	if !ok {
		svc.logger.Error("unexpected breaker response type")
		return nil, fmt.Errorf("unexpected response type from breaker")
	}

	svc.logger.Debug("grpc GetByID succeeded",
		"email", email,
		"file_id", fileId,
	)

	return &types.File{
		FileId:      file.Id,
		UploadId:    file.UploadId,
		OwnerEmail:  file.OwnerEmail,
		Name:        file.Name,
		Type:        file.Type,
		Size:        file.Size,
		TotalChunks: file.TotalChunks,
		Checksum:    file.Checksum,
		CreatedAt:   file.CreatedAt.AsTime(),
	}, nil
}

func (svc *FileServiceImpl) Delete(ctx context.Context, fileId, ownerEmail string) error {
	_, err := svc.breaker.Execute(func() (any, error) {
		grpcCtx, cancel := context.WithDeadline(ctx, time.Now().Add(2*time.Second))
		defer cancel()

		svc.logger.Info("circuit breaker state is ", svc.breaker.State())

		return svc.clientStub.DeleteFile(grpcCtx, &pb.FileDeleteRequest{
			FileId:     fileId,
			OwnerEmail: ownerEmail,
		})
	})
	if err != nil {
		svc.logger.Error("grpc DeleteFile failed",
			"file_id", fileId,
			"error", err,
		)
		return fmt.Errorf("delete file via grpc: %w", err)
	}

	svc.logger.Debug("grpc DeleteFile succeeded",
		"file_id", fileId,
	)
	return nil
}

func (svc *FileServiceImpl) GetDownloadURL(ctx context.Context, fileId string, ttl time.Duration) (string, error) {
	replyRaw, err := svc.breaker.Execute(func() (any, error) {
		grpcCtx, cancel := context.WithDeadline(ctx, time.Now().Add(2*time.Second))
		defer cancel()

		svc.logger.Info("circuit breaker state is ", svc.breaker.State())

		return svc.clientStub.GetDownUrl(grpcCtx, &pb.FileDownUrlRequest{
			FileId: fileId,
			Ttl:    durationpb.New(ttl),
		})
	})
	if err != nil {
		svc.logger.Error("grpc GetDownloadURL failed",
			"file_id", fileId,
			"error", err,
		)
		return "", fmt.Errorf("delete file via grpc: %w", err)
	}

	reply, ok := replyRaw.(*pb.FileDownUrlReply)
	if !ok {
		svc.logger.Error("unexpected breaker response type")
		return "", fmt.Errorf("unexpected response type from breaker")
	}

	svc.logger.Debug("grpc GetDownloadURL succeeded",
		"file_id", fileId,
	)
	return reply.Url, nil
}
