//+build linux openbsd netbsd freebsd plan9 solaris darwin dragonfly

package s3push

import "github.com/rakyll/magicmime"

type unixMimeGuesser struct{}

func newMimeGuesser() mimeTypeGuesser {
	return unixMimeGuesser{}
}

func (g unixMimeGuesser) init() error {
	return magicmime.Open(magicmime.MAGIC_MIME_TYPE | magicmime.MAGIC_SYMLINK | magicmime.MAGIC_ERROR)
}

func (g unixMimeGuesser) close() {
	magicmime.Close()
}

func (g unixMimeGuesser) mimeTypeFromPath(path string) (string, error) {
	return magicmime.TypeByFile(path)
}
