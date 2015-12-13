package main

import (
    "os"
    //"io"
    "fmt"
    "github.com/speedata/gogit"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3/s3manager"
    "path/filepath"
)

const REF_S3_PUSH string = "LAST_S3_PUSH"

type Repository struct {
    GitRepo         *gogit.Repository
    HeadCommit      *gogit.Commit
    LastPushCommit  *gogit.Commit
    UnpushedFiles   []string
    Config          RepoConfig
    S3Uploader      S3Uploader
}

type RepoConfig struct {
    S3Region        string
    S3Bucket        string
}

func OpenRepository() (*Repository, error) {
    repo := new(Repository)

    wd, err := os.Getwd()
    if err != nil {
        return nil, err
    }

    path := filepath.Join(wd, ".git")
    if _, err := os.Stat(path); os.IsNotExist(err) {
        return nil, err
    }

    gitRepo, err := gogit.OpenRepository(path)
    if err != nil {
        return nil, err
    }
    repo.GitRepo = gitRepo

    return repo, nil
}

func (repo *Repository) FindRelevantCommits() error {
    headRef, err := repo.GitRepo.LookupReference("HEAD")
    if err != nil {
        return err
    }
    
    headCommit, err := repo.GitRepo.LookupCommit(headRef.Target())
    if err != nil {
        return err
    }
    repo.HeadCommit = headCommit

    lastPushRef, err := repo.GitRepo.LookupReference(REF_S3_PUSH)
    if err != nil {
        return nil
    }

    lastPushCommit, err := repo.GitRepo.LookupCommit(lastPushRef.Target())
    if err != nil {
        return nil
    }
    repo.LastPushCommit = lastPushCommit

    return nil
}

func (repo *Repository) ModifiedFilesInCommit(dirname string, te *gogit.TreeEntry) int {
    filePath := filepath.Join(dirname, te.Name)

    if _, err := os.Stat(filePath); err == nil {
        repo.UnpushedFiles = append(repo.UnpushedFiles, filePath)
    }

    return 0;
}

func (repo *Repository) FindUnpushedModifiedFiles() {
    if repo.HeadCommit.Id().Equal(repo.LastPushCommit.Id()) {
        return
    }

    currentCommit := repo.HeadCommit;

    for currentCommit != nil && currentCommit.ParentCount() > 0 {
        currentCommit.Tree.Walk(repo.ModifiedFilesInCommit)

        if repo.LastPushCommit != nil && repo.LastPushCommit.Id() == currentCommit.Id() {
            break;
        }

        currentCommit = currentCommit.Parent(0)
    }
}

type S3Uploader struct {
    BucketName      *string
    S3Uploader      *s3manager.Uploader
}

func InitS3Uploader(config RepoConfig) *S3Uploader {
    uploader := new(S3Uploader)
    uploader.BucketName = aws.String(config.S3Bucket)

    s3config := aws.Config{Region: aws.String(config.S3Region)}
    s3uploader := s3manager.NewUploader(session.New(&s3config))
    uploader.S3Uploader = s3uploader

    return uploader
}

func (uploader S3Uploader) UploadFile(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return err
    }

    result, err := uploader.S3Uploader.Upload(&s3manager.UploadInput{
        Body: file,
        Bucket: uploader.BucketName,
        Key: aws.String(path),
    })

    if err != nil {
        return err
    }

    fmt.Println(result.Location)
    return nil
}

func main() {
    repo, err := OpenRepository()
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }

    if err := repo.FindRelevantCommits(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }

    repo.FindUnpushedModifiedFiles();

    if len(repo.UnpushedFiles) == 0 {
        fmt.Println("No modified files to push")
        os.Exit(0)
    }

    config := RepoConfig{S3Region: "eu-west-1", S3Bucket: "git-s3-push-test"}
    uploader := InitS3Uploader(config)

    for _, filePath := range repo.UnpushedFiles {
        fmt.Println("Uploading: " + filePath)
        err := uploader.UploadFile(filePath)
        if err != nil {
            fmt.Println(err)
            os.Exit(1)
        }
    }
}