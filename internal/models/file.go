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

type IndexQuality struct {
	GeneratedOrCacheDirsIgnored  bool              `json:"generatedOrCacheDirsIgnored"`
	IgnoredDirCounts             map[string]int    `json:"ignoredDirCounts,omitempty"`
	LargeFilesSkipped            int               `json:"largeFilesSkipped"`
	BinaryFilesSkipped           int               `json:"binaryFilesSkipped"`
	UnresolvedInternalImports    int               `json:"unresolvedInternalImports"`
	InternalAliasImportsResolved int               `json:"internalAliasImportsResolved"`
	UnresolvedAliasImports       int               `json:"unresolvedAliasImports"`
	Warnings                     []string          `json:"warnings,omitempty"`
	SkippedLargeFiles            []SkippedFileInfo `json:"skippedLargeFiles,omitempty"`
	SkippedBinaryFiles           []SkippedFileInfo `json:"skippedBinaryFiles,omitempty"`
}

type SkippedFileInfo struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
	Reason    string `json:"reason"`
}
