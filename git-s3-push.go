package main

import (
    "os"
    "os/exec"
    "bufio"
    "fmt"
    "flag"
    "encoding/json"
    "io/ioutil"
    "github.com/speedata/gogit"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3/s3manager"
    "github.com/deckarep/golang-set"
    "path/filepath"
)

const REF_S3_PUSH string = "LAST_S3_PUSH"
const CONFIG_FILE_PATH string = ".git_s3_push"

type Repository struct {
    GitRepo         *gogit.Repository
    HeadCommit      *gogit.Commit
    LastPushCommit  *gogit.Commit
    UnpushedFiles   mapset.Set
    Config          RepoConfig
    S3Uploader      S3Uploader
}

type RepoConfig struct {
    S3Region        string
    S3Bucket        string
}

func OpenRepository() (*Repository, error) {
    repo := new(Repository)
    repo.UnpushedFiles = mapset.NewSet()

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

func (repo *Repository) ReadConfigFile() error {
    file, err := ioutil.ReadFile(CONFIG_FILE_PATH)
    if err != nil {
        return err
    }

    err = json.Unmarshal(file, &repo.Config)
    if err != nil {
        return err
    }
    return nil
}

func (repo Repository) SaveConfigToFile() error {
    jsonData, err := json.Marshal(repo.Config)
    if err != nil {
        return err
    }

    err = ioutil.WriteFile(CONFIG_FILE_PATH, jsonData, 0644)
    if err != nil {
        return err
    }

    return nil
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

func (repo *Repository) ReadGitModifiedFiles(scanner *bufio.Scanner, stop chan bool)  {
    for scanner.Scan() {
        repo.UnpushedFiles.Add(scanner.Text())
    }

    stop <- true
}

func (repo *Repository) FindCommitModifiedFiles(commit *gogit.Commit) error {
    cmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "--root", commit.Id().String())
    out, err := cmd.StdoutPipe()
    if err != nil {
        return err
    }

    err = cmd.Start()
    if err != nil {
        return err
    }

    scanner := bufio.NewScanner(out)

    stop := make(chan bool)
    go repo.ReadGitModifiedFiles(scanner, stop)
    <-stop
    cmd.Wait()

    return nil
}

func (repo *Repository) FindUnpushedModifiedFiles() error {
    queue := []*gogit.Commit{};
    visited := mapset.NewSet();

    currentCommit := repo.HeadCommit;
    for currentCommit != nil && currentCommit.ParentCount() > 0 {
        if repo.LastPushCommit != nil && repo.LastPushCommit.Id().Equal(currentCommit.Id()) {
            break;
        }

        err := repo.FindCommitModifiedFiles(currentCommit)
        if err != nil {
            return err
        }

        for i := 0; i < currentCommit.ParentCount(); i++ {
            parentCommit := currentCommit.Parent(i)
            if !visited.Contains(parentCommit) {
                queue = append(queue, parentCommit)
            }
        }

        if len(queue) < 1 {
            break;
        }

        currentCommit = queue[0]
        queue = queue[1:]
    }
    
    return nil
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
    repo.ReadConfigFile()

    flag.StringVar(&repo.Config.S3Bucket, "b", repo.Config.S3Bucket, "Destination S3 bucket name")
    flag.StringVar(&repo.Config.S3Region, "r", repo.Config.S3Region, "AWS region of destination bucket")
    saveConfig := flag.Bool("save", false, "Save destination region/bucket to config file")
    flag.Parse()

    if repo.Config.S3Bucket == "" {
        flag.Usage()
        os.Exit(1)
    } else if (repo.Config.S3Region == "") {
        flag.Usage()
        os.Exit(1)
    } else if (*saveConfig) {
        err := repo.SaveConfigToFile()
        if err != nil {
            fmt.Println("WARNING: Failed to save config to file: ", err)
        }
    }

    if err := repo.FindRelevantCommits(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }

    repo.FindUnpushedModifiedFiles();

    if repo.UnpushedFiles.Cardinality() == 0 {
        fmt.Println("No modified files to push")
        os.Exit(0)
    }

    uploader := InitS3Uploader(repo.Config)

    for filePath := range repo.UnpushedFiles.Iter() {
        fmt.Println("Uploading: " + filePath.(string))
        err := uploader.UploadFile(filePath.(string))
        if err != nil {
            fmt.Println(err)
            os.Exit(1)
        }
    }
}