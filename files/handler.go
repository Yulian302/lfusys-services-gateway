package files

import (
	"fmt"

	"github.com/Yulian302/lfusys-services-commons/errors"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	fileService services.FileService

	Logger logger.Logger
}

func NewFileHandler(fileService services.FileService, l logger.Logger) *FileHandler {
	return &FileHandler{
		fileService: fileService,
		Logger:      l,
	}
}

func (h *FileHandler) GetFiles(c *gin.Context) {
	email := c.GetString("email")
	resp, err := h.fileService.GetFiles(c.Request.Context(), email)
	if err != nil {
		h.Logger.Error("failed to get files",
			"email", email,
			"error", err,
		)
		errors.InternalServerErrorResponse(c, "could not get files")
		return
	}

	h.Logger.Info("files fetched successfully",
		"email", email,
		"count", len(resp.Files),
	)

	responses.JSONData(c, 200, resp)
}

func (h *FileHandler) DeleteFile(c *gin.Context) {
	fileId := c.Param("fileId")
	if fileId == "" {
		h.Logger.Warn("file id not specified by user")
		errors.BadRequestResponse(c, "file id must be specified")
		return
	}

	email, _ := c.Get("email")
	err := h.fileService.Delete(c.Request.Context(), fileId)
	if err != nil {
		h.Logger.Error("failed to get files",
			"email", email,
			"error", err,
		)
		errors.InternalServerErrorResponse(c, "could not delete file")
		return
	}

	h.Logger.Info(fmt.Sprintf("file %s deleted successfully", fileId),
		"email", email,
	)

	responses.JSONDeleted(c, "success")
}
