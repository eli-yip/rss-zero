package file

import (
	"context"
	"io"
)

// File interface is for file related services.
type File interface {
	// SaveStream method will take a name, save the stream to the file service
	SaveStream(path string, readCloser io.ReadCloser, size int64) error
	// GetStream method will take a name, and return the stream from the file service
	GetStream(string) (io.ReadCloser, error)
	// AssetsDomain is a getter for the assets domain
	AssetsDomain() string
	// Delete method will take a name, and delete the file from the file service
	Delete(string) error
	// Check existance of a file
	Exist(string) (bool, error)
	// Size returns the byte size of a stored object.
	Size(string) (int64, error)
}

// ContextFile 供迁移为存储请求设置截止时间，不改变已有调用方的接口。
type ContextFile interface {
	File
	GetStreamContext(context.Context, string) (io.ReadCloser, error)
	SaveStreamContext(context.Context, string, io.ReadCloser, int64) error
}
