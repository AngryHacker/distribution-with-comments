package storage

import (
	"errors"
	"fmt"

	"github.com/docker/distribution/context"
	storageDriver "github.com/docker/distribution/registry/storage/driver"
)

// SkipDir is used as a return value from onFileFunc to indicate that
// the directory named in the call is to be skipped. It is not returned
// as an error by any function.
// 应该忽略的目录
var ErrSkipDir = errors.New("skip this directory")

// WalkFn is called once per file by Walk
// If the returned error is ErrSkipDir and fileInfo refers
// to a directory, the directory will not be entered and Walk
// will continue the traversal.  Otherwise Walk will return
// 作为被 Walk 调用的函数
type WalkFn func(fileInfo storageDriver.FileInfo) error

// Walk traverses a filesystem defined within driver, starting
// from the given path, calling f on each file
// 对 driver 中定义的文件系统从 from 开始遍历，并对每个文件调用 f 函数 
func Walk(ctx context.Context, driver storageDriver.StorageDriver, from string, f WalkFn) error {
	children, err := driver.List(ctx, from)
	if err != nil {
		return err
	}
	for _, child := range children {
		fileInfo, err := driver.Stat(ctx, child)
		if err != nil {
			return err
		}
		err = f(fileInfo)
		skipDir := (err == ErrSkipDir)
		if err != nil && !skipDir {
			return err
		}

		if fileInfo.IsDir() && !skipDir {
			Walk(ctx, driver, child, f)
		}
	}
	return nil
}

// pushError formats an error type given a path and an error
// and pushes it to a slice of errors
// 格式化 error 和 path 信息， 并添加到 errors 数组切片上
func pushError(errors []error, path string, err error) []error {
	return append(errors, fmt.Errorf("%s: %s", path, err))
}
