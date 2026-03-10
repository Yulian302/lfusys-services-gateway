package uploads

import (
	error "errors"
	"net/http"

	"github.com/Yulian302/lfusys-services-commons/errors"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/Yulian302/lfusys-services-gateway/uploads/types"
	"github.com/gin-gonic/gin"
)

type UploadsHandler struct {
	uploadsService services.UploadsService

	Logger logger.Logger
}

func NewUploadsHandler(uploadsService services.UploadsService, l logger.Logger) *UploadsHandler {
	return &UploadsHandler{
		uploadsService: uploadsService,
		Logger:         l,
	}
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
		h.Logger.Warn("start upload failed",
			"reason", "not_authenticated",
		)
		errors.UnauthorizedResponse(c, "user not authenticated")
		return
	}

	var uploadReq types.UploadRequest
	if err := c.ShouldBindJSON(&uploadReq); err != nil {
		h.Logger.Warn("start upload failed",
			"email", email,
			"reason", "bad_request",
		)
		errors.BadRequestResponse(c, err.Error())
		return
	}

	uploadResp, err := h.uploadsService.StartUpload(c.Request.Context(), email, uploadReq)
	if err != nil {
		if error.Is(err, errors.ErrFileSizeExceeded) || error.Is(err, errors.ErrFileSizeInvalid) {
			h.Logger.Warn("start upload failed",
				"email", email,
				"file_size", uploadReq.FileSize,
				"reason", "file_size_invalid",
			)
			errors.BadRequestResponse(c, "file cannot be larger than 10GB")
		} else if error.Is(err, errors.ErrSessionConflict) {
			h.Logger.Warn("start upload failed",
				"email", email,
				"reason", "session_conflict",
			)
			errors.ConflictResponse(c, "upload session already exists")
		} else if error.Is(err, errors.ErrServiceUnavailable) {
			h.Logger.Error("start upload failed",
				"email", email,
				"reason", "service_unavailable",
			)
			errors.ServiceUnavailableResponse(c, "upload service unavailable")
		} else {
			h.Logger.Error("start upload failed",
				"email", email,
				"error", err,
			)
			errors.InternalServerErrorResponse(c, err.Error())
		}
		return
	}

	h.Logger.Info("upload started successfully",
		"email", email,
		"upload_id", uploadResp.UploadId,
		"total_chunks", uploadResp.TotalChunks,
	)
	c.JSON(http.StatusOK, types.UploadResponse{
		TotalChunks: uploadResp.TotalChunks,
		UploadId:    uploadResp.UploadId,
	})
}

func (h *UploadsHandler) GetUploadStatus(c *gin.Context) {
	uploadId := c.Param("uploadId")
	if uploadId == "" {
		h.Logger.Warn("get upload status failed",
			"reason", "upload_id_missing",
		)
		errors.BadRequestResponse(c, "upload id is required")
		return
	}

	resp, err := h.uploadsService.GetUploadStatus(c.Request.Context(), uploadId)
	if err != nil {
		if error.Is(err, errors.ErrGrpcFailed) {
			h.Logger.Error("get upload status failed",
				"upload_id", uploadId,
				"reason", "grpc_failed",
			)
			errors.InternalServerErrorResponse(c, "grpc failed")
		} else if error.Is(err, errors.ErrServiceUnavailable) {
			h.Logger.Error("get upload status failed",
				"upload_id", uploadId,
				"reason", "service_unavailable",
			)
			errors.ServiceUnavailableResponse(c, "upload service unavailable")
		} else if error.Is(err, errors.ErrSessionNotFound) {
			h.Logger.Warn("get upload status failed",
				"upload_id", uploadId,
				"reason", "session_not_found",
			)
			errors.ForbiddenResponse(c, "upload session not found")
		} else {
			h.Logger.Error("get upload status failed",
				"upload_id", uploadId,
				"error", err,
			)
			errors.InternalServerErrorResponse(c, err.Error())
		}
		return
	}

	h.Logger.Debug("upload status retrieved successfully",
		"upload_id", uploadId,
	)
	c.JSON(http.StatusOK, resp)
}

func (h *UploadsHandler) GetUploadedChunks(c *gin.Context) {
	uploadId := c.Param("uploadId")
	if uploadId == "" {
		h.Logger.Warn("get uploaded chunks failed",
			"reason", "upload_id_missing",
		)
		errors.BadRequestResponse(c, "upload id is required")
		return
	}

	resp, err := h.uploadsService.GetUploadedChunks(c.Request.Context(), uploadId)
	if err != nil {
		if error.Is(err, errors.ErrGrpcFailed) {
			h.Logger.Error("get uploaded chunks status failed",
				"upload_id", uploadId,
				"reason", "grpc_failed",
			)
			errors.InternalServerErrorResponse(c, "grpc failed")
		} else if error.Is(err, errors.ErrServiceUnavailable) {
			h.Logger.Error("get uploaded chunks status failed",
				"upload_id", uploadId,
				"reason", "service_unavailable",
			)
			errors.ServiceUnavailableResponse(c, "upload service unavailable")
		} else if error.Is(err, errors.ErrSessionNotFound) {
			h.Logger.Warn("get uploaded chunks status failed",
				"upload_id", uploadId,
				"reason", "session_not_found",
			)
			errors.ForbiddenResponse(c, "upload session not found")
		} else {
			h.Logger.Error("get uploaded chunks status failed",
				"upload_id", uploadId,
				"error", err,
			)
			errors.InternalServerErrorResponse(c, err.Error())
		}
		return
	}

	h.Logger.Debug("uploaded chunks retrieved successfully",
		"upload_id", uploadId,
	)
	c.JSON(http.StatusOK, resp)
}
