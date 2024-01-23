package file

import (
	"context"
	"errors"
	"io"
	"path/filepath"

	gomime "github.com/cubewise-code/go-mime"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

type FileServiceMinio struct {
	MinioClient  *minio.Client
	bucketName   string
	assetsDomain string
	logger       *zap.Logger
}

type MinioConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
	AssetsPrefix    string
}

func NewFileServiceMinio(minioConfig MinioConfig, logger *zap.Logger) (*FileServiceMinio, error) {
	minioClient, err := minio.New(minioConfig.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioConfig.AccessKeyID, minioConfig.SecretAccessKey, ""),
		Secure: minioConfig.UseSSL,
	})
	if err != nil {
		logger.Error("Failed to init minio", zap.Error(err))
		return nil, err
	}

	return &FileServiceMinio{
		MinioClient:  minioClient,
		bucketName:   minioConfig.BucketName,
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

	ext := filepath.Ext(objectKey)
	contentType := gomime.TypeByExtension(ext)
	if ext == "" {
		contentType = "application/octet-stream"
	}

	var info minio.UploadInfo
	info, err = s.MinioClient.PutObject(context.Background(),
		s.bucketName,
		objectKey,
		stream,
		size,
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		return err
	}
	s.logger.Info("Successfully uploaded object to minio",
		zap.String("bucket", info.Bucket),
		zap.String("key", info.Key),
		zap.String("type", contentType),
		zap.Int64("size", info.Size),
	)

	return err
}

func (s *FileServiceMinio) Get(objectKey string) (stream io.ReadCloser, err error) {
	ctx := context.Background()
	o, err := s.MinioClient.GetObject(ctx, s.bucketName, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (s *FileServiceMinio) AssetsDomain() (url string) { return s.assetsDomain }
