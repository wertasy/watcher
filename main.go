package main

import (
	"archive/tar"
	"io"
	"io/fs"
	"log"
	"os"
	"path"

	"github.com/ulikunitz/xz/lzma"
)

func main() {
	// boot.PowerOn()
	baseDir := "tmp"
	file, err := os.Open("v2fly-core.tar.lzma")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	lr, err := lzma.NewReader(file)
	if err != nil {
		log.Fatal(err)
	}

	tr := tar.NewReader(lr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		path := path.Join(baseDir, hdr.Name)
		mode := hdr.FileInfo().Mode()
		foo(tr, path, mode)
	}
}

func foo(src io.Reader, path string, mode fs.FileMode) {
	if mode.IsDir() {
		err := os.MkdirAll(path, mode.Perm())
		if err != nil {
			log.Fatal(err)
		}
	}

	if mode.IsRegular() {
		dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
		if err != nil {
			log.Fatal(err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			log.Fatal(err)
		}
	}
}
