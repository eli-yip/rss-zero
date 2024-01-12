package file

import (
	"context"
	"errors"
	"io"

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

type MinioConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
	AssetsDomain    string
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
		minioClient:  minioClient,
		bucketName:   minioConfig.BucketName,
		assetsDomain: minioConfig.AssetsDomain,
		logger:       logger,
	}, nil
}

func (s *FileServiceMinio) SaveStream(objectKey string, stream io.ReadCloser, size int64) (err error) {
	if stream == nil {
		return errors.New("no body")
	}
	defer stream.Close()

	var info minio.UploadInfo
	info, err = s.minioClient.PutObject(context.Background(),
		s.bucketName,
		objectKey,
		stream,
		size,
		minio.PutObjectOptions{ContentType: "application/octet-stream"},
	)
	if err != nil {
		return err
	}
	s.logger.Info("Successfully uploaded bytes: ",
		zap.String("bucket", info.Bucket),
		zap.String("key", info.Key),
		zap.Int64("size", info.Size),
	)

	return err
}

func (s *FileServiceMinio) Get(objectKey string) (stream io.ReadCloser, err error) {
	o, err := s.minioClient.GetObject(context.Background(), s.bucketName, objectKey, minio.GetObjectOptions{})
	return o, err
}

func (s *FileServiceMinio) AssetsDomain() (url string) { return s.assetsDomain }
