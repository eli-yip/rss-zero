package file

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/eli-yip/rss-zero/pkg/log"
)

func TestNewFileServiceMinio(t *testing.T) {
	// Test cases for initializing FileServiceMinio
	logger := log.NewLogger()
	minioConfig := MinioConfig{
		Endpoint:        os.Getenv("MINIO_ENDPOINT"),
		AccessKeyID:     os.Getenv("MINIO_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("MINIO_SECRET_ACCESS_KEY"),
		UseSSL:          true,
		BucketName:      "rss",
		AssetsPrefix:    "test.com",
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
		AssetsPrefix:    "test.com",
	}

	minioService, err := NewFileServiceMinio(minioConfig, logger)
	if err != nil {
		t.Errorf("Failed to initialize FileServiceMinio: %v", err)
	}

	path := filepath.Join("testdata", "test.wav")
	file, err := os.Open(path)
	if err != nil {
		t.Errorf("Failed to open file: %v", err)
	}
	fileStat, err := file.Stat()
	if err != nil {
		t.Errorf("Failed to get file stat: %v", err)
	}
	size := fileStat.Size()

	err = minioService.SaveStream("test.wav", file, size)
	if err != nil {
		t.Errorf("Failed to save stream: %v", err)
	}
}

func TestGet(t *testing.T) {
	logger := log.NewLogger()
	minioConfig := MinioConfig{
		Endpoint:        os.Getenv("MINIO_ENDPOINT"),
		AccessKeyID:     os.Getenv("MINIO_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("MINIO_SECRET_ACCESS_KEY"),
		UseSSL:          true,
		BucketName:      "rss",
		AssetsPrefix:    "test.com",
	}

	minioService, err := NewFileServiceMinio(minioConfig, logger)
	if err != nil {
		t.Errorf("Failed to initialize FileServiceMinio: %v", err)
	}

	stream, err := minioService.Get("test.wav")
	if err != nil {
		t.Errorf("Failed to get stream: %v", err)
	}
	defer stream.Close()

	bytes, err := io.ReadAll(stream)
	if err != nil {
		t.Errorf("Failed to read stream: %v", err)
	}

	fmt.Println(len(bytes))

	// save to file
	path := filepath.Join("testdata", "test-result.wav")
	file, err := os.Create(path)
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}
	defer file.Close()

	_, err = file.Write(bytes)
	if err != nil {
		t.Errorf("Failed to write to file: %v", err)
	}

	stat, _ := file.Stat()
	fmt.Println(stat.Size())

	t.Logf("File saved")
}
