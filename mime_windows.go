//+build windows

package s3push

import (
    "errors"
    "mime"
    "path"
)

type windowsMimeGuesser struct{}

func newMimeGuesser() mimeTypeGuesser {
	return windowsMimeGuesser{}
}

func (g windowsMimeGuesser) init() error {
	return nil
}

func (g windowsMimeGuesser) close() {}

func (g windowsMimeGuesser) mimeTypeFromPath(filePath string) (string, error) {
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
