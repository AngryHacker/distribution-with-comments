// Package base provides a base implementation of the storage driver that can
// be used to implement common checks. The goal is to increase the amount of
// code sharing.
//
// The canonical approach to use this class is to embed in the exported driver
// struct such that calls are proxied through this implementation. First,
// declare the internal driver, as follows:
//
// 	type driver struct { ... internal ...}
//
// The resulting type should implement StorageDriver such that it can be the
// target of a Base struct. The exported type can then be declared as follows:
//
// 	type Driver struct {
// 		Base
// 	}
//
// Because Driver embeds Base, it effectively implements Base. If the driver
// needs to intercept a call, before going to base, Driver should implement
// that method. Effectively, Driver can intercept calls before coming in and
// driver implements the actual logic.
//
// To further shield the embed from other packages, it is recommended to
// employ a private embed struct:
//
// 	type baseEmbed struct {
// 		base.Base
// 	}
//
// Then, declare driver to embed baseEmbed, rather than Base directly:
//
// 	type Driver struct {
// 		baseEmbed
// 	}
//
// The type now implements StorageDriver, proxying through Base, without
// exporting an unnecessary field.
package base

import (
	"io"

	"github.com/docker/distribution/context"
	storagedriver "github.com/docker/distribution/registry/storage/driver"
)

// Base provides a wrapper around a storagedriver implementation that provides
// common path and bounds checking.
// Base 用于包裹一个 StorageDriver, 在调用 StorageDriver 的相应函数前，
// 会先调用 Base 的函数
type Base struct {
	storagedriver.StorageDriver
}

// GetContent wraps GetContent of underlying storage driver.
// GetContent 函数的预处理
func (base *Base) GetContent(ctx context.Context, path string) ([]byte, error) {
	ctx, done := context.WithTrace(ctx)
	defer done("%s.GetContent(%q)", base.Name(), path)
	
	// 检查路径正则
	if !storagedriver.PathRegexp.MatchString(path) {
		return nil, storagedriver.InvalidPathError{Path: path}
	}

	return base.StorageDriver.GetContent(ctx, path)
}

// PutContent wraps PutContent of underlying storage driver.
// PutContent 函数的预处理
func (base *Base) PutContent(ctx context.Context, path string, content []byte) error {
	ctx, done := context.WithTrace(context.Background())
	defer done("%s.PutContent(%q)", base.Name(), path)
	
	// 检查路径正则
	if !storagedriver.PathRegexp.MatchString(path) {
		return storagedriver.InvalidPathError{Path: path}
	}

	return base.StorageDriver.PutContent(ctx, path, content)
}

// ReadStream wraps ReadStream of underlying storage driver.、
// ReadStream 的预处理
func (base *Base) ReadStream(ctx context.Context, path string, offset int64) (io.ReadCloser, error) {
	ctx, done := context.WithTrace(context.Background())
	defer done("%s.ReadStream(%q, %d)", base.Name(), path, offset)
	
	// 检查偏移量
	if offset < 0 {
		return nil, storagedriver.InvalidOffsetError{Path: path, Offset: offset}
	}
	
	// 检查路径正则
	if !storagedriver.PathRegexp.MatchString(path) {
		return nil, storagedriver.InvalidPathError{Path: path}
	}

	return base.StorageDriver.ReadStream(ctx, path, offset)
}

// WriteStream wraps WriteStream of underlying storage driver.
// WriteStream 的预处理
func (base *Base) WriteStream(ctx context.Context, path string, offset int64, reader io.Reader) (nn int64, err error) {
	ctx, done := context.WithTrace(ctx)
	defer done("%s.WriteStream(%q, %d)", base.Name(), path, offset)
	
	// 检查偏移量
	if offset < 0 {
		return 0, storagedriver.InvalidOffsetError{Path: path, Offset: offset}
	}
	
	// 检查路径正则
	if !storagedriver.PathRegexp.MatchString(path) {
		return 0, storagedriver.InvalidPathError{Path: path}
	}

	return base.StorageDriver.WriteStream(ctx, path, offset, reader)
}

// Stat wraps Stat of underlying storage driver.
// Stat 的预处理
func (base *Base) Stat(ctx context.Context, path string) (storagedriver.FileInfo, error) {
	ctx, done := context.WithTrace(ctx)
	defer done("%s.Stat(%q)", base.Name(), path)
	
	// 检查路径正则
	if !storagedriver.PathRegexp.MatchString(path) {
		return nil, storagedriver.InvalidPathError{Path: path}
	}

	return base.StorageDriver.Stat(ctx, path)
}

// List wraps List of underlying storage driver.
// List 的预处理
func (base *Base) List(ctx context.Context, path string) ([]string, error) {
	ctx, done := context.WithTrace(ctx)
	defer done("%s.List(%q)", base.Name(), path)
	
	// 检查路径正则
	if !storagedriver.PathRegexp.MatchString(path) && path != "/" {
		return nil, storagedriver.InvalidPathError{Path: path}
	}

	return base.StorageDriver.List(ctx, path)
}

// Move wraps Move of underlying storage driver.
// Move 的预处理
func (base *Base) Move(ctx context.Context, sourcePath string, destPath string) error {
	ctx, done := context.WithTrace(ctx)
	defer done("%s.Move(%q, %q", base.Name(), sourcePath, destPath)
	
	// 检查路径正则
	if !storagedriver.PathRegexp.MatchString(sourcePath) {
		return storagedriver.InvalidPathError{Path: sourcePath}
	} else if !storagedriver.PathRegexp.MatchString(destPath) {
		return storagedriver.InvalidPathError{Path: destPath}
	}

	return base.StorageDriver.Move(ctx, sourcePath, destPath)
}

// Delete wraps Delete of underlying storage driver.
// Delete 的预处理
func (base *Base) Delete(ctx context.Context, path string) error {
	ctx, done := context.WithTrace(ctx)
	defer done("%s.Delete(%q)", base.Name(), path)
	
	// 检查路径正则
	if !storagedriver.PathRegexp.MatchString(path) {
		return storagedriver.InvalidPathError{Path: path}
	}

	return base.StorageDriver.Delete(ctx, path)
}

// URLFor wraps URLFor of underlying storage driver.
// URLFor 的预处理
func (base *Base) URLFor(ctx context.Context, path string, options map[string]interface{}) (string, error) {
	ctx, done := context.WithTrace(ctx)
	defer done("%s.URLFor(%q)", base.Name(), path)
	
	// 检查路径正则
	if !storagedriver.PathRegexp.MatchString(path) {
		return "", storagedriver.InvalidPathError{Path: path}
	}

	return base.StorageDriver.URLFor(ctx, path, options)
}
