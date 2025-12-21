package types

type UploadResponse struct {
	TotalChunks uint32   `json:"total_chunks"`
	UploadUrls  []string `json:"upload_urls"`
	UploadId    string   `json:"upload_id"`
}
