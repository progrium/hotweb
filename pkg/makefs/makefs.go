package makefs

import (
	"os"
	"path"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/afero/mem"
)

type Fs struct {
	afero.Fs
	transforms map[string][]transform
}

type transform struct {
	srcExt string
	fn     transformFn
}

type transformFn func(fs afero.Fs, dst, src string) ([]byte, error)

func New(readFs, writeFs afero.Fs) *Fs {
	return &Fs{
		Fs: afero.NewCopyOnWriteFs(
			afero.NewReadOnlyFs(readFs),
			writeFs,
		),
		transforms: make(map[string][]transform),
	}
}

func (f *Fs) Register(dstExt, srcExt string, fn transformFn) {
	f.transforms[dstExt] = append(f.transforms[dstExt], transform{
		srcExt: srcExt,
		fn:     fn,
	})
}

func (f *Fs) ensureTransforms(name string) afero.File {
	transforms, ok := f.transforms[path.Ext(name)]
	if !ok {
		return nil
	}
	for _, transform := range transforms {
		srcFile := strings.ReplaceAll(name, path.Ext(name), transform.srcExt)
		srcExists, err := afero.Exists(f.Fs, srcFile)
		if err != nil {
			panic(err)
		}
		if srcExists {
			b, err := transform.fn(f.Fs, name, srcFile)
			if err != nil {
				panic(err)
			}
			f := mem.NewFileHandle(mem.CreateFile(name))
			_, err = f.Write(b)
			if err != nil {
				panic(err)
			}
			f.Seek(0, 0)
			return f
		}
	}
	return nil
}

func (f *Fs) Open(name string) (afero.File, error) {
	if tf := f.ensureTransforms(name); tf != nil {
		return tf, nil
	}
	return f.Fs.Open(name)
}

func (f *Fs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if tf := f.ensureTransforms(name); tf != nil {
		return tf, nil
	}
	return f.Fs.OpenFile(name, flag, perm)
}

func (f *Fs) Stat(name string) (os.FileInfo, error) {
	if tf := f.ensureTransforms(name); tf != nil {
		return tf.Stat()
	}
	return f.Fs.Stat(name)
}
