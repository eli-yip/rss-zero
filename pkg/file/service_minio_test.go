package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eli-yip/zsxq-parser/pkg/log"
)

func TestNewFileServiceMinio(t *testing.T) {
	// Test cases for initializing FileServiceMinio
	logger := log.NewLogger()
	minioConfig := MinioConfig{
		Endpoint:        os.Getenv("MINIO_ENDPOINT"),
		AccessKeyID:     os.Getenv("MINIO_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("MINIO_SECRET_ACCESS_KEY"),
		UseSSL:          true,
		BucketName:      "test",
		AssetsDomain:    "test.com",
	}

	_, err := NewFileServiceMinio(minioConfig, logger)
	if err != nil {
		t.Errorf("Failed to initialize FileServiceMinio: %v", err)
	}
}

func TestSaveStream(t *testing.T) {
	// Test cases for initializing FileServiceMinio
	logger := log.NewLogger()
	minioConfig := MinioConfig{
		Endpoint:        os.Getenv("MINIO_ENDPOINT"),
		AccessKeyID:     os.Getenv("MINIO_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("MINIO_SECRET_ACCESS_KEY"),
		UseSSL:          true,
		BucketName:      "rss",
		AssetsDomain:    "test.com",
	}

	minioService, err := NewFileServiceMinio(minioConfig, logger)
	if err != nil {
		t.Errorf("Failed to initialize FileServiceMinio: %v", err)
	}

	path := filepath.Join("testdata", "abc.txt")
	file, err := os.Open(path)
	if err != nil {
		t.Errorf("Failed to open file: %v", err)
	}
	fileStat, err := file.Stat()
	if err != nil {
		t.Errorf("Failed to get file stat: %v", err)
	}
	size := fileStat.Size()

	err = minioService.SaveStream("test.txt", file, size)
	if err != nil {
		t.Errorf("Failed to save stream: %v", err)
	}
}
