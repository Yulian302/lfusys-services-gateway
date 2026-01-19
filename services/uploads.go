package services

import (
	"context"
	"fmt"
	"time"

	pb "github.com/Yulian302/lfusys-services-commons/api"
	"github.com/Yulian302/lfusys-services-commons/errors"
	"github.com/Yulian302/lfusys-services-gateway/store"
	uploadstypes "github.com/Yulian302/lfusys-services-gateway/uploads/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UploadsService interface {
	StartUpload(ctx context.Context, email string, fileSize int64) (*uploadstypes.UploadResponse, error)
	GetUploadStatus(ctx context.Context, uploadID string) (*uploadstypes.UploadStatusResponse, error)
}

type UploadsServiceImpl struct {
	uploadsStore store.UploadsStore
	clientStub   pb.UploaderClient
	maxFileSize  int64
}

func NewUploadsService(uploadsStore store.UploadsStore, cb pb.UploaderClient) *UploadsServiceImpl {
	return &UploadsServiceImpl{
		uploadsStore: uploadsStore,
		clientStub:   cb,
		maxFileSize:  10 * 1024 * 1024 * 1024,
	}
}

func (s *UploadsServiceImpl) StartUpload(ctx context.Context, email string, fileSize int64) (*uploadstypes.UploadResponse, error) {
	if fileSize <= 0 {
		return nil, fmt.Errorf("%w", errors.ErrFileSizeInvalid)
	}

	if fileSize > s.maxFileSize {
		return nil, fmt.Errorf("%w", errors.ErrFileSizeExceeded)
	}

	exists, err := s.uploadsStore.FindExisting(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrInternalServer, err)
	}

	if exists {
		return nil, fmt.Errorf("%w", errors.ErrSessionConflict)
	}
	grpcContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	res, err := s.clientStub.StartUpload(grpcContext, &pb.UploadRequest{
		UserEmail: email,
		FileSize:  uint64(fileSize),
	})
	if err != nil {
		if status.Code(err) == codes.ResourceExhausted {
			return nil, fmt.Errorf("%w", errors.ErrFileSizeExceeded)
		}
		if status.Code(err) == codes.Unavailable {
			return nil, fmt.Errorf("%w", errors.ErrServiceUnavailable)
		}
		return nil, fmt.Errorf("%w", errors.ErrGrpcFailed)
	}

	return &uploadstypes.UploadResponse{
		TotalChunks: res.TotalChunks,
		UploadUrls:  res.UploadUrls,
		UploadId:    res.UploadId,
	}, nil
}

func (s *UploadsServiceImpl) GetUploadStatus(ctx context.Context, uploadID string) (*uploadstypes.UploadStatusResponse, error) {
	uploadStatusOut, err := s.clientStub.GetUploadStatus(ctx, &pb.UploadID{
		UploadId: uploadID,
	})
	if err != nil {
		if status.Code(err) == codes.ResourceExhausted {
			return nil, fmt.Errorf("%w", errors.ErrFileSizeExceeded)
		}
		if status.Code(err) == codes.Unavailable {
			return nil, fmt.Errorf("%w", errors.ErrServiceUnavailable)
		}
		return nil, fmt.Errorf("could not get upload status: %w", err)
	}
	return &uploadstypes.UploadStatusResponse{
		Status:   uploadStatusOut.Status,
		Progress: uploadStatusOut.Progress,
		Message:  uploadStatusOut.Message,
	}, nil
}
