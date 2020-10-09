package s3push

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const cannedAclPublicRead = "public-read"
const cannedAclPrivate = "private"

// S3Uploader manages S3 uploads to a specific bucket
type S3Uploader struct {
	bucketName  *string
	prefix      string
	public      bool
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
	uploader.bucketName = aws.String(config.S3Bucket)
	uploader.public = config.Public
	uploader.prefix = config.Prefix

	if len(uploader.prefix) > 0 && uploader.prefix[len(uploader.prefix)-1:] != "/" {
		uploader.prefix = uploader.prefix + "/"
	}

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

	var acl string
	if uploader.public {
		acl = cannedAclPublicRead
	} else {
		acl = cannedAclPrivate
	}

	result, err := uploader.s3Uploader.Upload(&s3manager.UploadInput{
		Body:        file,
		Bucket:      uploader.bucketName,
		Key:         aws.String(uploader.prefix + path),
		ContentType: &contentType,
		ACL:         &acl,
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
