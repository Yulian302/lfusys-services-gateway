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

	logger logger.Logger
}

func NewFileHandler(fileService services.FileService, l logger.Logger) *FileHandler {
	return &FileHandler{
		fileService: fileService,
		logger:      l,
	}
}

func (h *FileHandler) GetFiles(c *gin.Context) {
	email := c.GetString("email")
	resp, err := h.fileService.GetFiles(c.Request.Context(), email)
	if err != nil {
		h.logger.Error("failed to get files",
			"email", email,
			"error", err,
		)
		errors.InternalServerErrorResponse(c, "could not get files")
		return
	}

	h.logger.Info("files fetched successfully",
		"email", email,
		"count", len(resp.Files),
	)

	responses.JSONData(c, 200, resp)
}

func (h *FileHandler) DeleteFile(c *gin.Context) {
	fileId := c.Param("fileId")
	if fileId == "" {
		h.logger.Warn("file id not specified by user")
		errors.BadRequestResponse(c, "file id must be specified")
		return
	}

	email, _ := c.Get("email")
	err := h.fileService.Delete(c.Request.Context(), fileId)
	if err != nil {
		h.logger.Error("failed to get files",
			"email", email,
			"error", err,
		)
		errors.InternalServerErrorResponse(c, "could not delete file")
		return
	}

	h.logger.Info(fmt.Sprintf("file %s deleted successfully", fileId),
		"email", email,
	)

	responses.JSONDeleted(c, "success")
}
