package uploads

import (
	"net/http"

	pb "github.com/Yulian302/lfusys-services-commons/api"
	"github.com/Yulian302/lfusys-services-commons/errors"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/gin-gonic/gin"
)

type UploadsHandler struct {
	clientStub pb.UploaderClient
	store      store.UploadsStore
}

func NewUploadsHandler(cb pb.UploaderClient, store store.UploadsStore) *UploadsHandler {
	return &UploadsHandler{
		clientStub: cb,
		store:      store,
	}
}

type UploadRequest struct {
	FileSize uint64 `json:"file_size" binding:"required"`
}

type UploadResponse struct {
	TotalChunks uint32   `json:"total_chunks"`
	UploadUrls  []string `json:"upload_urls"`
	UploadId    string   `json:"upload_id"`
}

// StartUpload godoc
// @Summary      Start an upload session
// @Description  Start an upload session by getting a file size
// @Tags         uploads
// @Accept       json
// @Produce      json
// @Param        request   body      UploadRequest  true  "Upload request"
// @Success      200  {object}  UploadResponse "Upload info"
// @Failure      401  {object}  HTTPError "Not authenticated"
// @Failure      400  {object}  HTTPError "Bad request params"
// @Failure      500  {object}  HTTPError
// @Router       /uploads/start [post]
func (h *UploadsHandler) StartUpload(ctx *gin.Context) {
	email := ctx.GetString("email")
	if email == "" {
		errors.Unauthorized(ctx, "user not authenticated")
		return
	}
	var uploadReq UploadRequest
	if err := ctx.ShouldBindJSON(&uploadReq); err != nil {
		errors.BadRequestError(ctx, err.Error())
		return
	}

	if uploadReq.FileSize > 10*1024*1024*1024 {
		errors.BadRequestError(ctx, "file size exceeds 10GB limit")
		return
	}
	exists, err := h.store.FindExisting(ctx, email)
	if err != nil {
		errors.InternalServerError(ctx, "failed to check existing session")
		return
	}
	if exists {
		errors.JSONError(ctx, http.StatusConflict, "active upload session already exists")
		return
	}

	res, err := h.clientStub.StartUpload(ctx, &pb.UploadRequest{
		UserEmail: email,
		FileSize:  uint64(uploadReq.FileSize),
	})
	if err != nil {
		errors.InternalServerError(ctx, "could not receive response from server")
		return
	}
	ctx.JSON(http.StatusOK, UploadResponse{
		TotalChunks: res.TotalChunks,
		UploadUrls:  res.UploadUrls,
		UploadId:    res.UploadId,
	})
}
