package s3push

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/s3"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const cannedAclPublicRead = "public-read"
const cannedAclPrivate = "private"

// S3Uploader manages S3 uploads to a specific bucket
type S3Uploader struct {
	endpoint 	string
	bucketName  *string
	prefix      string
	public      bool
	s3Uploader  *s3manager.Uploader
	s3Svc       *s3.S3
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
	uploader.endpoint = config.Endpoint

	if len(uploader.prefix) > 0 && uploader.prefix[len(uploader.prefix)-1:] != "/" {
		uploader.prefix = uploader.prefix + "/"
	}

	s3config := aws.Config{Region: aws.String(config.S3Region)}

	if uploader.endpoint != "" {
		s3config = aws.Config{Region: aws.String(config.S3Region), Endpoint: aws.String(uploader.endpoint)}
	}

	s3Session, err := session.NewSession(&s3config)

	if err != nil {
		return nil, err
	}

	uploader.s3Uploader = s3manager.NewUploader(s3Session)
	uploader.s3Svc = s3.New(s3Session)

	uploader.mimeGuesser = newMimeGuesser()
	err = uploader.mimeGuesser.init()
	if err != nil {
		return nil, err
	}

	return uploader, nil
}

func (uploader S3Uploader) deleteFile(path string) error {
	key := aws.String(uploader.prefix + path)

	_, err := uploader.s3Svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: uploader.bucketName,
		Key:    key,
	})

	if err != nil {
		return err
	}

	err = uploader.s3Svc.WaitUntilObjectNotExists(&s3.HeadObjectInput{
		Bucket: uploader.bucketName,
		Key:    key,
	})

	return err
}

// UploadFile uploads a file to S3
func (uploader S3Uploader) UploadFile(path string) error {

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Uploading a file which does not exist means delete it
		return uploader.deleteFile(path)
	}

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
