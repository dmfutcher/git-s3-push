package s3push

import (
    "errors"
    "mime"
    "path"
)

type mimeGuesser struct{}

func newMimeGuesser() mimeTypeGuesser {
	return mimeGuesser{}
}

func (g mimeGuesser) init() error {
	return nil
}

func (g mimeGuesser) close() {}

func (g mimeGuesser) mimeTypeFromPath(filePath string) (string, error) {
	ext := path.Ext(filePath)
    if ext == "" {
        return "", errors.New("No file extension")
    }

    mimeType := mime.TypeByExtension(ext)
    if mimeType == "" {
        return "", errors.New("Mime type unknown")
    }

    return mimeType, nil
}
