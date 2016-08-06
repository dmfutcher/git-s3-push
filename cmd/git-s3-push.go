package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bobbo/git-s3-push"
)

func main() {
	repo, err := s3push.OpenRepository()
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

	uploader, err := s3push.InitS3Uploader(repo.Config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer uploader.Close()

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
