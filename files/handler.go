package files

import (
	"github.com/Yulian302/lfusys-services-commons/errors"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	fileService services.FileService
}

func NewFileHandler(fileService services.FileService) *FileHandler {
	return &FileHandler{
		fileService: fileService,
	}
}

func (h *FileHandler) GetFiles(c *gin.Context) {
	email := c.GetString("email")
	if email == "" {
		errors.ForbiddenResponse(c, "could not validate user authenticity")
		return
	}
	resp, err := h.fileService.GetFiles(c, email)
	if err != nil {
		errors.InternalServerErrorResponse(c, "could not get files")
		return
	}

	responses.JSONData(c, 200, resp)
}
