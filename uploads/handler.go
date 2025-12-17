package uploads

import (
	"net/http"
	"strconv"

	pb "github.com/Yulian302/lfusys-services-commons/api"
	"github.com/Yulian302/lfusys-services-commons/errors"
	"github.com/gin-gonic/gin"
)

type UploadsHandler struct {
	clientStub pb.UploaderClient
}

func NewUploadsHandler(cb pb.UploaderClient) *UploadsHandler {
	return &UploadsHandler{
		clientStub: cb,
	}
}

type UploadRequest struct {
	FileSize string `json:"file_size" binding:"required"`
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

	fileSize, err := strconv.ParseUint(uploadReq.FileSize, 10, 64)
	if err != nil {
		errors.InternalServerError(ctx, "failed to check existing session")
		return
	}
	res, err := h.clientStub.StartUpload(ctx, &pb.UploadRequest{
		UserEmail: email,
		FileSize:  uint64(fileSize),
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
