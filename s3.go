package s3push

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// S3Uploader manages S3 uploads to a specific bucket
type S3Uploader struct {
	BucketName  *string
	s3Uploader  *s3manager.Uploader
	mimeGuesser mimeTypeGuesser
}

type mimeTypeGuesser interface {
	init() error
	mimeTypeFromPath(string) (string, error)
	close()
}

// InitS3Uploader initializes a new S3Uploader
func InitS3Uploader(config repoConfig) (*S3Uploader, error) {
	uploader := new(S3Uploader)
	uploader.BucketName = aws.String(config.S3Bucket)

	s3config := aws.Config{Region: aws.String(config.S3Region)}
	s3uploader := s3manager.NewUploader(session.New(&s3config))
	uploader.s3Uploader = s3uploader

	uploader.mimeGuesser = newMimeGuesser()
	err := uploader.mimeGuesser.init()
	if err != nil {
		return nil, err
	}

	return uploader, nil
}

// UploadFile uploads a file to S3
func (uploader S3Uploader) UploadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	contentType, err := uploader.mimeGuesser.mimeTypeFromPath(path)
	if err != nil {
		fmt.Println("Couldn't automatically determine content type of ", path, err)
		contentType = "text/plain"
	}

	result, err := uploader.s3Uploader.Upload(&s3manager.UploadInput{
		Body:        file,
		Bucket:      uploader.BucketName,
		Key:         aws.String(path),
		ContentType: &contentType,
	})

	if err != nil {
		return err
	}

	fmt.Println(result.Location)
	return nil
}

// Close cleans up the uploader
func (uploader S3Uploader) Close() {
	uploader.mimeGuesser.close()
}
