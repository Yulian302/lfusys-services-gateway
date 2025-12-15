package uploads

import (
	"net/http"
	"strconv"

	pb "github.com/Yulian302/lfusys-services-commons/api"
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

type HTTPError struct {
	Error string `json:"error" example:"error message"`
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
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}
	var uploadReq UploadRequest
	if err := ctx.ShouldBindJSON(&uploadReq); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid fields",
		})
		return
	}

	fileSize, err := strconv.ParseUint(uploadReq.FileSize, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid file_size",
		})
		return
	}
	res, err := h.clientStub.StartUpload(ctx, &pb.UploadRequest{
		UserEmail: email,
		FileSize:  uint64(fileSize),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not receive response from server",
		})
		return
	}
	ctx.JSON(http.StatusOK, UploadResponse{
		TotalChunks: res.TotalChunks,
		UploadUrls:  res.UploadUrls,
		UploadId:    res.UploadId,
	})
}
