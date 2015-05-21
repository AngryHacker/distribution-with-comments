package driver

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/distribution/context"
)

// Version is a string representing the storage driver version, of the form
// Major.Minor.
// The registry must accept storage drivers with equal major version and greater
// minor version, but may not be compatible with older storage driver versions.
// storage driver 的版本， 接受 major 一样的大版本，大于 minor 的小版本
type Version string

// Major returns the major (primary) component of a version.
func (version Version) Major() uint {
	majorPart := strings.Split(string(version), ".")[0]
	major, _ := strconv.ParseUint(majorPart, 10, 0)
	return uint(major)
}

// Minor returns the minor (secondary) component of a version.
func (version Version) Minor() uint {
	minorPart := strings.Split(string(version), ".")[1]
	minor, _ := strconv.ParseUint(minorPart, 10, 0)
	return uint(minor)
}

// CurrentVersion is the current storage driver Version.
// 当前版本
const CurrentVersion Version = "0.1"

// StorageDriver defines methods that a Storage Driver must implement for a
// filesystem-like key/value object storage.
// 所有 Storage Driver 必须实现的方法， key/value 的文件系统
type StorageDriver interface {
	// Name returns the human-readable "name" of the driver, useful in error
	// messages and logging. By convention, this will just be the registration
	// name, but drivers may provide other information here.
	// 返回友好的 storage 名字
	Name() string

	// GetContent retrieves the content stored at "path" as a []byte.
	// This should primarily be used for small objects.
	// 根据 path 中返回 []byte 类型的数据， 主要用于小文件
	GetContent(ctx context.Context, path string) ([]byte, error)

	// PutContent stores the []byte content at a location designated by "path".
	// This should primarily be used for small objects.
	// 在 path 中存储 []byte 数据， 用于小文件
	PutContent(ctx context.Context, path string, content []byte) error

	// ReadStream retrieves an io.ReadCloser for the content stored at "path"
	// with a given byte offset.
	// May be used to resume reading a stream by providing a nonzero offset.
	// 在指定的 path 返回一个偏移 offset 大小的 io.ReadCloser, 可以用 offset 来指定继续未完成的 reading
	ReadStream(ctx context.Context, path string, offset int64) (io.ReadCloser, error)

	// WriteStream stores the contents of the provided io.ReadCloser at a
	// location designated by the given path.
	// May be used to resume writing a stream by providing a nonzero offset.
	// The offset must be no larger than the CurrentSize for this path.
	// 从 reader 中读取的内容写入 path， 可以用 offset 来指定偏移量， 但不能超过
	// path 的当前大小， 方便前一次未完成的写入
	WriteStream(ctx context.Context, path string, offset int64, reader io.Reader) (nn int64, err error)

	// Stat retrieves the FileInfo for the given path, including the current
	// size in bytes and the creation time.
	// 返回 path 的信息， 包括大小和创建时间
	Stat(ctx context.Context, path string) (FileInfo, error)

	// List returns a list of the objects that are direct descendants of the
	//given path.
	// 返回 path 的直接子节点， 类似 ls
	List(ctx context.Context, path string) ([]string, error)

	// Move moves an object stored at sourcePath to destPath, removing the
	// original object.
	// Note: This may be no more efficient than a copy followed by a delete for
	// many implementations.
	// 移动文件到新位置， 会删除原有文件
	Move(ctx context.Context, sourcePath string, destPath string) error

	// Delete recursively deletes all objects stored at "path" and its subpaths.
	// 递归删除文件
	Delete(ctx context.Context, path string) error

	// URLFor returns a URL which may be used to retrieve the content stored at
	// the given path, possibly using the given options.
	// May return an ErrUnsupportedMethod in certain StorageDriver
	// implementations.
	// 返回 path 的 url 形式
	URLFor(ctx context.Context, path string, options map[string]interface{}) (string, error)
}

// PathRegexp is the regular expression which each file path must match. A
// file path is absolute, beginning with a slash and containing a positive
// number of path components separated by slashes, where each component is
// restricted to lowercase alphanumeric characters or a period, underscore, or
// hyphen.
// 设定路径必须满足的正则
var PathRegexp = regexp.MustCompile(`^(/[A-Za-z0-9._-]+)+$`)

// ErrUnsupportedMethod may be returned in the case where a StorageDriver implementation does not support an optional method.
// storage 不支持某方法而抛出的错误
var ErrUnsupportedMethod = errors.New("unsupported method")

// PathNotFoundError is returned when operating on a nonexistent path.
// 路径不存在
type PathNotFoundError struct {
	Path string
}

func (err PathNotFoundError) Error() string {
	return fmt.Sprintf("Path not found: %s", err.Path)
}

// InvalidPathError is returned when the provided path is malformed.
// 不合格的路径
type InvalidPathError struct {
	Path string
}

func (err InvalidPathError) Error() string {
	return fmt.Sprintf("Invalid path: %s", err.Path)
}

// InvalidOffsetError is returned when attempting to read or write from an
// invalid offset.
// 不合格的偏移量
type InvalidOffsetError struct {
	Path   string
	Offset int64
}

func (err InvalidOffsetError) Error() string {
	return fmt.Sprintf("Invalid offset: %d for path: %s", err.Offset, err.Path)
}
