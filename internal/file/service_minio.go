package file

import (
	"context"
	"errors"
	"io"
	"path/filepath"

	gomime "github.com/cubewise-code/go-mime"
	"github.com/eli-yip/rss-zero/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

type FileServiceMinio struct {
	minioClient  *minio.Client
	bucketName   string
	assetsDomain string
	logger       *zap.Logger
}

func NewFileServiceMinio(minioConfig config.MinioConfig, logger *zap.Logger) (File, error) {
	minioClient, err := minio.New(minioConfig.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioConfig.AccessKeyID, minioConfig.SecretAccessKey, ""),
		Secure: true,
	})
	if err != nil {
		return nil, err
	}

	return &FileServiceMinio{
		minioClient:  minioClient,
		bucketName:   minioConfig.Bucket,
		assetsDomain: minioConfig.AssetsPrefix,
		logger:       logger,
	}, nil
}

func (s *FileServiceMinio) SaveStream(objectKey string, stream io.ReadCloser, size int64) (err error) {
	s.logger.Info("Start to save stream to minio", zap.String("key", objectKey))

	if stream == nil {
		return errors.New("no body")
	}
	defer stream.Close()

	contentType := s.getContentType(objectKey)

	var info minio.UploadInfo
	if info, err = s.minioClient.PutObject(
		context.Background(),
		s.bucketName,
		objectKey,
		stream,
		size,
		minio.PutObjectOptions{ContentType: contentType},
	); err != nil {
		return err
	}

	s.logger.Info("Upload to minio successfully",
		zap.String("bucket", info.Bucket),
		zap.String("key", info.Key),
		zap.String("type", contentType),
		zap.Int64("size", info.Size))

	return nil
}

// getContentType will return the content type based on the file extension.
// If the extension is not found, it will return "application/octet-stream".
func (s *FileServiceMinio) getContentType(objectKey string) (contentType string) {
	ext := filepath.Ext(objectKey)
	contentType = gomime.TypeByExtension(ext)
	if ext == "" {
		contentType = "application/octet-stream"
	}
	return contentType
}

func (s *FileServiceMinio) GetStream(objectKey string) (stream io.ReadCloser, err error) {
	return s.minioClient.GetObject(context.Background(), s.bucketName, objectKey, minio.GetObjectOptions{})
}

func (s *FileServiceMinio) AssetsDomain() (url string) { return s.assetsDomain }

func (s *FileServiceMinio) Delete(key string) (err error) {
	return s.minioClient.RemoveObject(context.Background(), s.bucketName, key, minio.RemoveObjectOptions{})
}

func (s *FileServiceMinio) Exist(key string) (exist bool, err error) {
	_, err = s.minioClient.StatObject(context.Background(), s.bucketName, key, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
