package main

import (
    "os"
    "fmt"
    "github.com/speedata/gogit"
    "path/filepath"
)

const REF_S3_PUSH string = "LAST_S3_PUSH"

type Repository struct {
    GitRepo         *gogit.Repository
    HeadCommit      *gogit.Commit
    LastPushCommit  *gogit.Commit
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

    fmt.Println(repo.LastPushCommit)
}