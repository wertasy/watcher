package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/heroku/docker-registry-client/registry"
	"github.com/itchio/lzma"
	"github.com/mholt/archiver"
	"github.com/opencontainers/go-digest"
)

//Manifest manifest.json
type Manifest struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

const baseDir = "/tmp/ubuntu/"

func untarlzma(name string) {
	file, err := os.Open(name)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	tr := tar.NewReader(lzma.NewReader(file))
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
		switch {
		case mode.IsDir():
			err = os.MkdirAll(path, mode)
		case mode.IsRegular():
			err = extract(tr, path, mode)
		}
		if err != nil {
			log.Fatal(err)
		}
	}
}

func main() {
	hub, err := registry.NewInsecure("http://localhost:5000", "", "")
	if err != nil {
		log.Fatal(err)
	}

	err = os.RemoveAll(baseDir)
	if err != nil {
		log.Fatal(err)
	}

	err = os.MkdirAll(baseDir, os.ModePerm|os.ModeDir)
	if err != nil {
		log.Fatal(err)
	}

	err = archiver.Unarchive("ubuntu.tar", baseDir)
	if err != nil {
		log.Fatal(err)
	}

	data, err := ioutil.ReadFile(path.Join(baseDir, "manifest.json"))
	if err != nil {
		log.Fatal(err)
	}

	var manifests []*Manifest
	err = json.Unmarshal(data, &manifests)
	if err != nil {
		log.Fatal(err)
	}

	log.Print("push image")
	for _, m := range manifests {
		m2 := schema2.Manifest{Versioned: schema2.SchemaVersion}
		for _, repoTag := range m.RepoTags {
			i := strings.IndexByte(repoTag, ':')
			repo, tag := repoTag[:i], repoTag[i+1:]
			log.Println("push layers")
			tag = "V6.6.6"
			wg := new(sync.WaitGroup)
			for _, layer := range m.Layers {
				wg.Add(1)
				go func(layer string) {
					layer = path.Join(baseDir, layer)
					desc, err := uploadBlob(hub, repo, layer)
					if err != nil {
						log.Fatal(err)
					}
					desc.MediaType = schema2.MediaTypeUncompressedLayer
					m2.Layers = append(m2.Layers, *desc)
					wg.Done()
				}(layer)
			}

			wg.Wait()
			log.Print("push config")
			path := path.Join(baseDir, m.Config)
			desc, err := uploadBlob(hub, repo, path)
			desc.MediaType = schema2.MediaTypeImageConfig
			if err != nil {
				log.Fatal(err)
			}

			log.Print("push manifest")
			m2.Config = *desc
			deserializedManifest, err := schema2.FromStruct(m2)
			if err != nil {
				log.Fatal(err)
			}

			err = hub.PutManifest(repo, tag, deserializedManifest)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func uploadBlob(hub *registry.Registry, repository, path string) (*distribution.Descriptor, error) {
	log.Println(repository, path)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	hash := sha256.New()
	hash.Write(data)
	digest := digest.NewDigest("sha256", hash)
	exists, err := hub.HasBlob(repository, digest)
	if err != nil {
		return nil, err
	}

	des := &distribution.Descriptor{
		Digest: digest,
		Size:   int64(len(data)),
	}

	if !exists {
		err = hub.UploadBlob(repository, digest, bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
	} else {
		log.Print("exists")
	}

	return des, nil
}

func extract(src io.Reader, path string, mode fs.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, src); err != nil {
		return err
	}

	return nil
}
