package types

import "time"

type File struct {
	FileId      string    `json:"file_id"`      // Unique file identifier
	UploadId    string    `json:"upload_id"`    // Corresponding upload id
	OwnerEmail  string    `json:"owner_email"`  // File owner email
	Size        uint64    `json:"file_size"`    // Size of a file
	TotalChunks uint32    `json:"total_chunks"` // Number of 5MB file chunks
	Checksum    string    `json:"checksum"`     // File checksum
	CreatedAt   time.Time `json:"created_at"`   // Time of creation
}

type FilesResponse struct {
	Files []*File
}
