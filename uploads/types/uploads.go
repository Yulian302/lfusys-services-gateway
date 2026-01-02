package types

type UploadResponse struct {
	TotalChunks uint32   `json:"total_chunks"`
	UploadUrls  []string `json:"upload_urls"`
	UploadId    string   `json:"upload_id"`
}

type UploadStatusResponse struct {
	Status   string `json:"status"`
	Progress uint32 `json:"progress"`
	Message  string `json:"message"`
}
