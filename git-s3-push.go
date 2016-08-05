package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/deckarep/golang-set"
	"github.com/speedata/gogit"
)

const refS3Push string = "refs/heads/s3-pushed"
const configFilePath string = ".git_s3_push"

type repository struct {
	GitRepo        *gogit.Repository
	HeadCommit     *gogit.Commit
	LastPushCommit *gogit.Commit
	UnpushedFiles  mapset.Set
	Config         repoConfig
	IgnoreRegexes  []*regexp.Regexp
	s3Uploader     s3Uploader
}

type repoConfig struct {
	S3Region      string
	S3Bucket      string
	Ignore        []string
	IncludeNonGit []string
}

func openRepository() (*repository, error) {
	repo := new(repository)
	repo.UnpushedFiles = mapset.NewSet()

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(wd, ".git")
	if _, err = os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}

	gitRepo, err := gogit.OpenRepository(path)
	if err != nil {
		return nil, err
	}
	repo.GitRepo = gitRepo

	return repo, nil
}

func (repo *repository) ReadConfigFile() error {
	file, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return err
	}

	err = json.Unmarshal(file, &repo.Config)
	if err != nil {
		return err
	}

	err = repo.CompileIgnoreRegexes()
	if err != nil {
		return err
	}

	return nil
}

func (repo *repository) CompileIgnoreRegexes() error {
	for _, regexStr := range repo.Config.Ignore {
		regexStr = strings.Replace(regexStr, "*", "(.*)", -1)
		regex, err := regexp.Compile(regexStr)
		if err != nil {
			return err
		}

		repo.IgnoreRegexes = append(repo.IgnoreRegexes, regex)
	}

	return nil
}

func (repo repository) SaveConfigToFile() error {
	jsonData, err := json.Marshal(repo.Config)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(configFilePath, jsonData, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (repo *repository) FindRelevantCommits() error {
	headRef, err := repo.GitRepo.LookupReference("HEAD")
	if err != nil {
		return err
	}

	headCommit, err := repo.GitRepo.LookupCommit(headRef.Target())
	if err != nil {
		return err
	}
	repo.HeadCommit = headCommit

	lastPushRef, err := repo.GitRepo.LookupReference(refS3Push)
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

func (repo *repository) ReadGitModifiedFiles(scanner *bufio.Scanner) {
	for scanner.Scan() {
		file := scanner.Text()

		if _, err := os.Stat(file); os.IsNotExist(err) {
			continue
		}

		matched := false
		for _, regex := range repo.IgnoreRegexes {
			if regex.Match([]byte(file)) {
				fmt.Println("Skipping file " + file + " matches ignore spec " + regex.String())
				matched = true
				break
			}
		}

		if !matched {
			repo.UnpushedFiles.Add(scanner.Text())
		}
	}
}

func (repo *repository) FindCommitModifiedFiles(commit *gogit.Commit) error {
	cmd := exec.Command("git", "show", "--name-only", "--oneline", commit.Id().String())
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(out)

	repo.ReadGitModifiedFiles(scanner)
	cmd.Wait()

	return nil
}

func (repo *repository) FindUnpushedModifiedFiles() error {
	queue := []*gogit.Commit{}
	visited := mapset.NewSet()

	currentCommit := repo.HeadCommit
	for currentCommit != nil {
		if repo.LastPushCommit != nil && repo.LastPushCommit.Id().Equal(currentCommit.Id()) {
			break
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
			break
		}

		currentCommit = queue[0]
		queue = queue[1:]
	}

	return nil
}

func (repo repository) UpdateGitLastPushRef() error {
	newLastPushRef := repo.HeadCommit.Id().String()
	cmd := exec.Command("git", "update-ref", refS3Push, newLastPushRef)

	err := cmd.Start()
	if err != nil {
		return err
	}

	cmd.Wait()
	return nil
}

type s3Uploader struct {
	BucketName *string
	s3Uploader *s3manager.Uploader
}

func initS3Uploader(config repoConfig) *s3Uploader {
	uploader := new(s3Uploader)
	uploader.BucketName = aws.String(config.S3Bucket)

	s3config := aws.Config{Region: aws.String(config.S3Region)}
	s3uploader := s3manager.NewUploader(session.New(&s3config))
	uploader.s3Uploader = s3uploader

	return uploader
}

func (uploader s3Uploader) UploadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	result, err := uploader.s3Uploader.Upload(&s3manager.UploadInput{
		Body:   file,
		Bucket: uploader.BucketName,
		Key:    aws.String(path),
	})

	if err != nil {
		return err
	}

	fmt.Println(result.Location)
	return nil
}

func main() {
	repo, err := openRepository()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	repo.ReadConfigFile()

	flag.StringVar(&repo.Config.S3Bucket, "b", repo.Config.S3Bucket, "Destination S3 bucket name")
	flag.StringVar(&repo.Config.S3Region, "r", repo.Config.S3Region, "AWS region of destination bucket")
	saveConfig := flag.Bool("save", false, "Save destination region/bucket to config file")
	forceNonTracked := flag.Bool("force-external", false, "Force the upload of files not tracked in git (IncludeNonGit files in config)")
	flag.Parse()

	if repo.Config.S3Bucket == "" {
		flag.Usage()
		os.Exit(1)
	} else if repo.Config.S3Region == "" {
		flag.Usage()
		os.Exit(1)
	} else if *saveConfig {
		err = repo.SaveConfigToFile()
		if err != nil {
			fmt.Println("WARNING: Failed to save config to file: ", err)
		}
	}

	if err = repo.FindRelevantCommits(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	repo.FindUnpushedModifiedFiles()

	if repo.UnpushedFiles.Cardinality() == 0 && !*forceNonTracked {
		fmt.Println("No modified files to push")
		os.Exit(0)
	}

	for _, includedFile := range repo.Config.IncludeNonGit {
		if _, err = os.Stat(includedFile); os.IsNotExist(err) {
			continue
		}

		repo.UnpushedFiles.Add(includedFile)
	}

	if repo.UnpushedFiles.Cardinality() == 0 {
		fmt.Println("No files to push")
		os.Exit(0)
	}

	uploader := initS3Uploader(repo.Config)

	for filePath := range repo.UnpushedFiles.Iter() {
		fmt.Println("Uploading: ", filePath.(string))
		err = uploader.UploadFile(filePath.(string))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	err = repo.UpdateGitLastPushRef()
	if err != nil {
		fmt.Println("Failed to update LAST_S3_PUSH ref with git: ", err)
	}
}
