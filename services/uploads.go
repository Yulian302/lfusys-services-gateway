package services

import (
	"context"
	"fmt"
	"time"

	pb "github.com/Yulian302/lfusys-services-commons/api/uploader/v1"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/Yulian302/lfusys-services-commons/errors"
	"github.com/Yulian302/lfusys-services-gateway/store"
	uploadstypes "github.com/Yulian302/lfusys-services-gateway/uploads/types"
	"github.com/sony/gobreaker/v2"
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
	breaker      *gobreaker.CircuitBreaker[*pb.UploadReply]
	maxFileSize  int64
	chunkSize    int64

	logger logger.Logger
}

func NewUploadsService(uploadsStore store.UploadsStore, cb pb.UploaderClient, breaker *gobreaker.CircuitBreaker[*pb.UploadReply], chunkSize int64, l logger.Logger) *UploadsServiceImpl {
	return &UploadsServiceImpl{
		uploadsStore: uploadsStore,
		clientStub:   cb,
		breaker:      breaker,
		maxFileSize:  10 * 1024 * 1024 * 1024,
		chunkSize:    chunkSize,
		logger:       l,
	}
}

func (s *UploadsServiceImpl) StartUpload(ctx context.Context, email string, fileSize int64) (*uploadstypes.UploadResponse, error) {
	if fileSize <= 0 {
		s.logger.Warn("start upload validation failed",
			"email", email,
			"file_size", fileSize,
			"reason", "invalid_file_size",
		)
		return nil, fmt.Errorf("%w", errors.ErrFileSizeInvalid)
	}

	if fileSize > s.maxFileSize {
		s.logger.Warn("start upload validation failed",
			"email", email,
			"file_size", fileSize,
			"max_file_size", s.maxFileSize,
			"reason", "file_size_exceeded",
		)
		return nil, fmt.Errorf("%w", errors.ErrFileSizeExceeded)
	}

	res, err := s.breaker.Execute(func() (*pb.UploadReply, error) {
		grpcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		return s.clientStub.StartUpload(grpcCtx, &pb.UploadRequest{
			UserEmail: email,
			FileSize:  uint64(fileSize),
			ChunkSize: uint64(s.chunkSize),
		})
	})

	if err != nil {
		if status.Code(err) == codes.ResourceExhausted {
			s.logger.Warn("start upload grpc failed",
				"email", email,
				"file_size", fileSize,
				"reason", "resource_exhausted",
			)
			return nil, fmt.Errorf("%w", errors.ErrFileSizeExceeded)
		}
		if status.Code(err) == codes.Unavailable {
			s.logger.Error("start upload grpc failed",
				"email", email,
				"file_size", fileSize,
				"reason", "service_unavailable",
			)
			return nil, fmt.Errorf("%w", errors.ErrServiceUnavailable)
		}
		s.logger.Error("start upload grpc failed",
			"email", email,
			"file_size", fileSize,
			"error", err,
		)
		return nil, fmt.Errorf("%w", errors.ErrGrpcFailed)
	}

	s.logger.Info("upload started",
		"email", email,
		"file_size", fileSize,
		"upload_id", res.UploadId,
		"total_chunks", res.TotalChunks,
	)

	return &uploadstypes.UploadResponse{
		TotalChunks: res.TotalChunks,
		UploadId:    res.UploadId,
	}, nil
}

func (s *UploadsServiceImpl) GetUploadStatus(ctx context.Context, uploadID string) (*uploadstypes.UploadStatusResponse, error) {
	uploadStatusOut, err := s.clientStub.GetUploadStatus(ctx, &pb.UploadID{
		UploadId: uploadID,
	})
	if err != nil {
		if status.Code(err) == codes.ResourceExhausted {
			s.logger.Warn("get upload status failed",
				"upload_id", uploadID,
				"reason", "resource_exhausted",
			)
			return nil, fmt.Errorf("%w", errors.ErrFileSizeExceeded)
		}
		if status.Code(err) == codes.Unavailable {
			s.logger.Error("get upload status failed",
				"upload_id", uploadID,
				"reason", "service_unavailable",
			)
			return nil, fmt.Errorf("%w", errors.ErrServiceUnavailable)
		}
		s.logger.Error("get upload status failed",
			"upload_id", uploadID,
			"error", err,
		)
		return nil, fmt.Errorf("could not get upload status: %w", err)
	}

	s.logger.Debug("upload status retrieved",
		"upload_id", uploadID,
		"status", uploadStatusOut.Status,
		"progress", uploadStatusOut.Progress,
	)

	return &uploadstypes.UploadStatusResponse{
		Status:   uploadStatusOut.Status,
		Progress: uploadStatusOut.Progress,
		Message:  uploadStatusOut.Message,
	}, nil
}
