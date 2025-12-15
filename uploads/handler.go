package uploads

import (
	"net/http"

	pb "github.com/Yulian302/lfusys-services-commons/api"
	"github.com/gin-gonic/gin"
)

type UploadsHandler struct {
	clientStub pb.GreeterClient
}

func NewUploadsHandler(cb pb.GreeterClient) *UploadsHandler {
	return &UploadsHandler{
		clientStub: cb,
	}
}

type HTTPError struct {
	Error string `json:"error" example:"error message"`
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
	res, err := h.clientStub.SayHello(ctx, &pb.HelloReq{
		Name: "Yulian",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not receive response from server",
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"message": res.Msg,
	})
}
