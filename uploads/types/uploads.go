package types

type UploadRequest struct {
	FileName string `json:"file_name" binding:"required"`
	FileType string `json:"file_type" binding:"required"`
	FileSize uint64 `json:"file_size" binding:"required"`
}

type UploadResponse struct {
	TotalChunks uint32 `json:"total_chunks"`
	UploadId    string `json:"upload_id"`
}

type UploadStatusResponse struct {
	Status   string `json:"status"`
	Progress uint32 `json:"progress"`
	Message  string `json:"message"`
}

type UploadedChunksResponse struct {
	Chunks []uint32 `json:"chunks"`
}
