package s3push

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckarep/golang-set"
	"github.com/speedata/gogit"
)

const refS3Push string = "refs/heads/s3-pushed"
const configFilePath string = ".git_s3_push"

// Repository represents a git-s3-push enabled git repository
type Repository struct {
	GitRepo        *gogit.Repository
	HeadCommit     *gogit.Commit
	LastPushCommit *gogit.Commit
	UnpushedFiles  mapset.Set
	Config         repoConfig
	IgnoreRegexes  []*regexp.Regexp
	s3Uploader     S3Uploader
}

type repoConfig struct {
	S3Region      string
	S3Bucket      string
	Public        bool
	Ignore        []string
	IncludeNonGit []string
}

// OpenRepository opens and initialises a 'git-s3-push' enabled git repository
func OpenRepository() (*Repository, error) {
	repo := new(Repository)
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

// ReadConfigFile reads .git_s3_push configuration file from repo
func (repo *Repository) ReadConfigFile() error {
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

// CompileIgnoreRegexes compiles the regexes in the Ignore configuration directive
func (repo *Repository) CompileIgnoreRegexes() error {
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

// SaveConfigToFile marshals the current configuration to JSON and saves it to .git_s3_push
func (repo Repository) SaveConfigToFile() error {
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

// FindRelevantCommits calls git to find commits not pushed to S3
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

// ReadGitModifiedFiles reads the git output describe files modified since last S3 push
func (repo *Repository) ReadGitModifiedFiles(scanner *bufio.Scanner) {
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

// FindCommitModifiedFiles finds files modified in given commit
func (repo *Repository) FindCommitModifiedFiles(commit *gogit.Commit) error {
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

// FindUnpushedModifiedFiles finds files that have been modified since last push to S3
func (repo *Repository) FindUnpushedModifiedFiles() error {
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

// UpdateGitLastPushRef sets the git-s3-push branch to the latest commit pushed
func (repo Repository) UpdateGitLastPushRef() error {
	newLastPushRef := repo.HeadCommit.Id().String()
	cmd := exec.Command("git", "update-ref", refS3Push, newLastPushRef)

	err := cmd.Start()
	if err != nil {
		return err
	}

	cmd.Wait()
	return nil
}
