package services

import (
	"context"
	"fmt"
	"time"

	pb "github.com/Yulian302/lfusys-services-commons/api/uploader/v1"
	"github.com/Yulian302/lfusys-services-commons/errors"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/Yulian302/lfusys-services-gateway/uploads/types"
	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UploadsService interface {
	StartUpload(ctx context.Context, email string, upload types.UploadRequest) (*types.UploadResponse, error)
	GetUploadStatus(ctx context.Context, uploadID string) (*types.UploadStatusResponse, error)
}

type UploadsServiceDeps struct {
	Client    pb.UploaderClient
	Breaker   *gobreaker.CircuitBreaker[*pb.UploadReply]
	ChunkSize int64

	Logger logger.Logger
}

type UploadsServiceImpl struct {
	clientStub  pb.UploaderClient
	breaker     *gobreaker.CircuitBreaker[*pb.UploadReply]
	maxFileSize uint64
	chunkSize   int64

	logger logger.Logger
}

func NewUploadsService(deps UploadsServiceDeps) *UploadsServiceImpl {
	return &UploadsServiceImpl{
		clientStub:  deps.Client,
		breaker:     deps.Breaker,
		maxFileSize: 10 * 1024 * 1024 * 1024,
		chunkSize:   deps.ChunkSize,
		logger:      deps.Logger,
	}
}

func (s *UploadsServiceImpl) StartUpload(ctx context.Context, email string, upload types.UploadRequest) (*types.UploadResponse, error) {
	fileSize := upload.FileSize
	if fileSize <= 0 {
		s.logger.Warn("start upload validation failed",
			"email", email,
			"file_name", upload.FileName,
			"file_type", upload.FileType,
			"file_size", fileSize,
			"reason", "invalid_file_size",
		)
		return nil, fmt.Errorf("%w", errors.ErrFileSizeInvalid)
	}

	if fileSize > s.maxFileSize {
		s.logger.Warn("start upload validation failed",
			"email", email,
			"file_name", upload.FileName,
			"file_type", upload.FileType,
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
			FileName:  upload.FileName,
			FileType:  upload.FileType,
			FileSize:  fileSize,
			ChunkSize: uint64(s.chunkSize),
		})
	})

	if err != nil {
		if status.Code(err) == codes.ResourceExhausted {
			s.logger.Warn("start upload grpc failed",
				"email", email,
				"file_name", upload.FileName,
				"file_type", upload.FileType,
				"file_size", fileSize,
				"reason", "resource_exhausted",
			)
			return nil, fmt.Errorf("%w", errors.ErrFileSizeExceeded)
		}
		if status.Code(err) == codes.Unavailable {
			s.logger.Error("start upload grpc failed",
				"email", email,
				"file_name", upload.FileName,
				"file_type", upload.FileType,
				"file_size", fileSize,
				"reason", "service_unavailable",
			)
			return nil, fmt.Errorf("%w", errors.ErrServiceUnavailable)
		}
		s.logger.Error("start upload grpc failed",
			"email", email,
			"file_name", upload.FileName,
			"file_type", upload.FileType,
			"file_size", fileSize,
			"error", err,
		)
		return nil, fmt.Errorf("%w", errors.ErrGrpcFailed)
	}

	s.logger.Info("upload started",
		"email", email,
		"file_name", upload.FileName,
		"file_type", upload.FileType,
		"file_size", fileSize,
		"upload_id", res.UploadId,
		"total_chunks", res.TotalChunks,
	)

	return &types.UploadResponse{
		TotalChunks: res.TotalChunks,
		UploadId:    res.UploadId,
	}, nil
}

func (s *UploadsServiceImpl) GetUploadStatus(ctx context.Context, uploadID string) (*types.UploadStatusResponse, error) {
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

	return &types.UploadStatusResponse{
		Status:   uploadStatusOut.Status,
		Progress: uploadStatusOut.Progress,
		Message:  uploadStatusOut.Message,
	}, nil
}
