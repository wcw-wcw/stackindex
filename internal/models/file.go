package models

type FileKind string

const (
	FileKindSource FileKind = "source"
	FileKindConfig FileKind = "config"
	FileKindTest   FileKind = "test"
	FileKindDoc    FileKind = "doc"
	FileKindOther  FileKind = "other"
)

type FileInfo struct {
	Path      string   `json:"path"`
	Ext       string   `json:"ext"`
	Language  string   `json:"language,omitempty"`
	SizeBytes int64    `json:"sizeBytes"`
	Kind      FileKind `json:"kind"`
	Hash      string   `json:"hash,omitempty"`
}
