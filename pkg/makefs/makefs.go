package makefs

import (
	"os"
	"path"
	"strings"

	"github.com/spf13/afero"
)

type Fs struct {
	afero.Fs

	transforms map[string][]transform
	os         bool
}

type transform struct {
	srcExt string
	fn     transformFn
}

type transformFn func(fs afero.Fs, dst, src string) error

func New(readFs, writeFs afero.Fs) *Fs {
	_, ok := readFs.(*afero.OsFs)
	return &Fs{
		Fs: afero.NewCopyOnWriteFs(
			afero.NewReadOnlyFs(readFs),
			writeFs,
		),
		transforms: make(map[string][]transform),
		os:         ok,
	}
}

func (f *Fs) Real() bool {
	return f.os
}

func (f *Fs) Register(dstExt, srcExt string, fn transformFn) {
	f.transforms[dstExt] = append(f.transforms[dstExt], transform{
		srcExt: srcExt,
		fn:     fn,
	})
}

func (f *Fs) ensureTransforms(name string) {
	transforms, ok := f.transforms[path.Ext(name)]
	if !ok {
		return
	}
	for _, transform := range transforms {
		srcFile := strings.ReplaceAll(name, path.Ext(name), transform.srcExt)
		srcExists, err := afero.Exists(f.Fs, srcFile)
		if err != nil {
			panic(err)
		}
		if srcExists {
			if err := transform.fn(f.Fs, name, srcFile); err != nil {
				panic(err)
			}
			return
		}
	}
}

func (f *Fs) Open(name string) (afero.File, error) {
	f.ensureTransforms(name)
	return f.Fs.Open(name)
}

func (f *Fs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	f.ensureTransforms(name)
	return f.Fs.OpenFile(name, flag, perm)
}

func (f *Fs) Stat(name string) (os.FileInfo, error) {
	f.ensureTransforms(name)
	return f.Fs.Stat(name)
}
