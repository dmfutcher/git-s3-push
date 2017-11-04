git-s3-push
===========
[![Build Status](https://travis-ci.org/bobbo/git-s3-push.svg?branch=master)](https://travis-ci.org/bobbo/git-s3-push)
[![Go Report Card](https://goreportcard.com/badge/github.com/bobbo/git-s3-push)](https://goreportcard.com/report/github.com/bobbo/git-s3-push)

**git-s3-push** is a tool to deploy git repositories to AWS S3 buckets. **git-s3-push** keeps track of which commits have been pushed and supports deploying only recently modified files.
It can be used for deploying [static websites hosted on S3](http://docs.aws.amazon.com/AmazonS3/latest/dev/WebsiteHosting.html), maintaining versioned bucket data or using S3 to backup git repositories.

## Features
- Simple method to deploy git repos to S3.
- Fast uploads by only uploading new commits.
- Automatically detects and sets the S3 content type of files.
- Can automatically make your files publicly available (private by default).
- Single binary, no [dependencies on language runtimes](https://github.com/schickling/git-s3).

## Installation

Grab a binary for your platform from the releases. Or check out the code, run `go get` and `go run git-s3-push.go`.

Git must be installed on your path.

## Usage
Authentication credentials are taken from the standard AWS environment variables. Bucket name and AWS region are supplied as arguments.

```$ export AWS_ACCESS_KEY_ID=<...>```

```$ export AWS_SECRET_ACCESS_KEY=<...>```

```$ git-s3-push -b my-bucket-name -r aws-region-1 -save```

The `-save` flag stores the bucket name and region so you can push to the same location by just running:


```$ git-s3-push```

The `-public` flag can be used to make the files uploaded to your bucket publicly readable. When running without the `-public` flag, pushed files are stored privately.

All usage options can be shown using the `-help` flag.

## Config
After using the `-save` flag, `git-s3-push` creates a JSON configuration file (`.git_s3_push`) storing bucket and region information. This file also includes other configuration directives that cannot be specified using flags:

- `Ignore`: Files in the git repo that *should not* be pushed. This could include source files (for example .coffee files), or any other file in the git repository you don't want pushed to the S3 bucket. Files are specified in a JSON list of regexes. For example: `"Ignore":["src/*.coffee"]`

- `IncludeNonGit`: Files not tracked by git that should be pushed to the destination bucket. Files are specified in a JSON list of paths. Paths can be absolute or relative to the root of the git repository.

## License
* MIT license. See the [LICENSE](https://github.com/bobbo/git-s3-push/blob/master/LICENSE) file.
