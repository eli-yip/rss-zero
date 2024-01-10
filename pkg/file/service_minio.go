package file

import (
	"context"
	"errors"
	"io"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type FileServiceMinio struct {
	minioClient  *minio.Client
	bucketName   string
	AssetsDomain string
}

type MinioConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
	AssetsDomain    string
}

func NewFileServiceMinio(minioConfig MinioConfig) *FileServiceMinio {
	minioClient, err := minio.New(minioConfig.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioConfig.AccessKeyID, minioConfig.SecretAccessKey, ""),
		Secure: minioConfig.UseSSL,
	})
	if err != nil {
		// TODO: Handle error with zap.
		log.Fatalln(err)
	}

	return &FileServiceMinio{
		minioClient:  minioClient,
		bucketName:   minioConfig.BucketName,
		AssetsDomain: minioConfig.AssetsDomain,
	}
}

func (s *FileServiceMinio) SaveStream(objectKey string, resp io.ReadCloser) (err error) {
	if resp == nil {
		return errors.New("no body")
	}

	defer resp.Close()

	_, err = s.minioClient.PutObject(context.Background(),
		s.bucketName,
		objectKey,
		resp,
		-1,
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return err
	}

	return nil
}

func (s *FileServiceMinio) Get(objectKey string) (stream io.ReadCloser, err error) {
	return s.minioClient.GetObject(context.Background(), s.bucketName, objectKey, minio.GetObjectOptions{})
}

func (s *FileServiceMinio) GetAssetsDomain() (url string) {
	return s.AssetsDomain
}
