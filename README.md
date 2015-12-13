git-s3-push
===========
**git-s3-push** is a tool to deploy git repositories to AWS S3 buckets. **git-s3-push** keeps track of which commits have been pushed and supports deploying only recently modified files. 
It can be used for deploying [static websites hosted on S3](http://docs.aws.amazon.com/AmazonS3/latest/dev/WebsiteHosting.html), maintaining versioned bucket data or using S3 to backup git repositories.

## Features
- Simple method to deploy git repos to S3.
- Fast uploads by only uploading new commits.
- Single binary, no [dependencies on language runtimes](https://github.com/schickling/git-s3)

## Usage
Authentication credentials are taken from the standard AWS environment variables. Bucket name and AWS region are supplied as arguments.
```$ export AWS_ACCESS_KEY_ID=<...>
$ export AWS_SECRET_ACCESS_KEY=<...>
$ git-s3-push -b my-bucket-name -r aws-region-1 -save```

The `-save` flag stores the bucket name and region so you can push to the same location by just running:
```$ git-s3-push```

## License 
* MIT license. See the [LICENSE](https://github.com/bobbo/git-s3-push/blob/master/LICENSE) file.
