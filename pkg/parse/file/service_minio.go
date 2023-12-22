package file

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/eli-yip/zsxq-parser/pkg/request"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type FileServiceMinio struct {
	requestService request.RequestIface
	minioClient    *minio.Client
	bucketName     string
}

type MinioConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
}

func NewFileServiceMinio(requestService request.RequestIface, minioConfig MinioConfig) *FileServiceMinio {
	minioClient, err := minio.New(minioConfig.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioConfig.AccessKeyID, minioConfig.SecretAccessKey, ""),
		Secure: minioConfig.UseSSL,
	})
	if err != nil {
		// TODO: Handle error with zap.
		log.Fatalln(err)
	}

	return &FileServiceMinio{
		requestService: requestService,
		minioClient:    minioClient,
		bucketName:     minioConfig.BucketName,
	}
}

type FileDownload struct {
	RespData struct {
		DownloadURL string `json:"download_url"`
	} `json:"resp_data"`
}

const ZsxqFileBaseURL = "https://api.zsxq.com/v2/files/%d/download_url"

func (s *FileServiceMinio) DownloadLink(fileID int) (link string, err error) {
	url := fmt.Sprintf(ZsxqFileBaseURL, fileID)

	resp, err := s.requestService.SendWithLimiter(url)
	if err != nil {
		return "", err
	}

	download := FileDownload{}
	if err = json.Unmarshal(resp, &download); err != nil {
		return "", err
	}

	return download.RespData.DownloadURL, nil
}

func (s *FileServiceMinio) Save(objectKey, downloadLink string) (err error) {
	resp, err := s.requestService.SendWithLimiterStream(downloadLink)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = s.minioClient.PutObject(context.Background(),
		s.bucketName,
		objectKey,
		resp.Body,
		-1,
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return err
	}

	return nil
}
