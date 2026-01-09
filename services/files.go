package services

import (
	"context"

	pb "github.com/Yulian302/lfusys-services-commons/api"
	"github.com/Yulian302/lfusys-services-gateway/files/types"
)

type FileService interface {
	GetFiles(ctx context.Context, email string) (*types.FilesResponse, error)
}

type FileServiceImpl struct {
	clientStub pb.UploaderClient
}

func NewFileServiceImpl(stub pb.UploaderClient) *FileServiceImpl {
	return &FileServiceImpl{
		clientStub: stub,
	}
}

func (svc *FileServiceImpl) GetFiles(ctx context.Context, email string) (*types.FilesResponse, error) {
	reply, err := svc.clientStub.GetFiles(ctx, &pb.UserInfo{
		Email: email,
	})
	if err != nil {
		return nil, err
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
