package uploads

import (
	error "errors"
	"net/http"

	"github.com/Yulian302/lfusys-services-commons/errors"
	"github.com/Yulian302/lfusys-services-gateway/services"
	uploadstypes "github.com/Yulian302/lfusys-services-gateway/uploads/types"
	"github.com/gin-gonic/gin"
)

type UploadsHandler struct {
	uploadsService services.UploadsService
}

func NewUploadsHandler(uploadsService services.UploadsService) *UploadsHandler {
	return &UploadsHandler{
		uploadsService: uploadsService,
	}
}

type UploadRequest struct {
	FileSize uint64 `json:"file_size" binding:"required"`
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
func (h *UploadsHandler) StartUpload(c *gin.Context) {
	email := c.GetString("email")
	if email == "" {
		errors.UnauthorizedResponse(c, "user not authenticated")
		return
	}

	var uploadReq UploadRequest
	if err := c.ShouldBindJSON(&uploadReq); err != nil {
		errors.BadRequestResponse(c, err.Error())
		return
	}

	uploadResp, err := h.uploadsService.StartUpload(c.Request.Context(), email, int64(uploadReq.FileSize))
	if err != nil {
		if error.Is(err, errors.ErrFileSizeExceeded) || error.Is(err, errors.ErrFileSizeInvalid) {
			errors.BadRequestResponse(c, "file cannot be larger than 10GB")
		} else if error.Is(err, errors.ErrSessionConflict) {
			errors.ConflictResponse(c, "upload session already exists")
		} else if error.Is(err, errors.ErrServiceUnavailable) {
			errors.ServiceUnavailableResponse(c, "upload service unavailable")
		} else {
			errors.InternalServerErrorResponse(c, err.Error())
		}
		return
	}
	c.JSON(http.StatusOK, uploadstypes.UploadResponse{
		TotalChunks: uploadResp.TotalChunks,
		UploadId:    uploadResp.UploadId,
	})
}

func (h *UploadsHandler) GetUploadStatus(c *gin.Context) {
	uploadId := c.Param("uploadId")
	if uploadId == "" {
		errors.BadRequestResponse(c, "upload id is required")
		return
	}

	resp, err := h.uploadsService.GetUploadStatus(c.Request.Context(), uploadId)
	if err != nil {
		if error.Is(err, errors.ErrGrpcFailed) {
			errors.InternalServerErrorResponse(c, "grpc failed")
		} else if error.Is(err, errors.ErrServiceUnavailable) {
			errors.ServiceUnavailableResponse(c, "upload service unavailable")
		} else if error.Is(err, errors.ErrSessionNotFound) {
			errors.ForbiddenResponse(c, "upload session not found")
		} else {
			errors.InternalServerErrorResponse(c, err.Error())
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}
