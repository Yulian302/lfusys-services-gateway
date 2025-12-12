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
