package client

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
)

var (
	// ErrLayerAlreadyExists is returned when attempting to create a layer with
	// a tarsum that is already in use.
	// layer 已存在
	ErrLayerAlreadyExists = fmt.Errorf("Layer already exists")

	// ErrLayerLocked is returned when attempting to write to a layer which is
	// currently being written to.
	// layer 正在被写入
	ErrLayerLocked = fmt.Errorf("Layer locked")
)

// ObjectStore is an interface which is designed to approximate the docker
// engine storage. This interface is subject to change to conform to the
// future requirements of the engine.
// 作为 docker 的存储引擎
type ObjectStore interface {
	// Manifest retrieves the image manifest stored at the given repository name
	// and tag
	// 返回 repository 中的 image manifest
	Manifest(name, tag string) (*manifest.SignedManifest, error)

	// WriteManifest stores an image manifest at the given repository name and
	// tag
	// 把 manifest 写入 respository
	WriteManifest(name, tag string, manifest *manifest.SignedManifest) error

	// Layer returns a handle to a layer for reading and writing
	// 返回对一个 layer 的钩子
	Layer(dgst digest.Digest) (Layer, error)
}

// Layer is a generic image layer interface.
// A Layer may not be written to if it is already complete.
// image layer 接口
type Layer interface {
	// Reader returns a LayerReader or an error if the layer has not been
	// written to or is currently being written to.
	// 对未写入的 layer 返回一个 LayerReader
	Reader() (LayerReader, error)

	// Writer returns a LayerWriter or an error if the layer has been fully
	// written to or is currently being written to.
	// 返回一个已被写入的 layer 或正在写入的 layer 的 LayerWriter
	Writer() (LayerWriter, error)

	// Wait blocks until the Layer can be read from.
	// 等待可读
	Wait() error
}

// LayerReader is a read-only handle to a Layer, which exposes the CurrentSize
// and full Size in addition to implementing the io.ReadCloser interface.
// 对一个 layer 只读
type LayerReader interface {
	io.ReadCloser

	// CurrentSize returns the number of bytes written to the underlying Layer
	CurrentSize() int

	// Size returns the full size of the underlying Layer
	Size() int
}

// LayerWriter is a write-only handle to a Layer, which exposes the CurrentSize
// and full Size in addition to implementing the io.WriteCloser interface.
// SetSize must be called on this LayerWriter before it can be written to.
// layer 只写. 
type LayerWriter interface {
	io.WriteCloser

	// CurrentSize returns the number of bytes written to the underlying Layer
	CurrentSize() int

	// Size returns the full size of the underlying Layer
	Size() int

	// SetSize sets the full size of the underlying Layer.
	// This must be called before any calls to Write
	// 在 write 之前必须调用
	SetSize(int) error
}

// memoryObjectStore is an in-memory implementation of the ObjectStore interface
// ObjectStore 的内存实现版本
type memoryObjectStore struct {
	// 锁
	mutex           *sync.Mutex
	// name:tag 到 manifest 的映射
	manifestStorage map[string]*manifest.SignedManifest
	layerStorage    map[digest.Digest]Layer
}

// 返回 manifest
func (objStore *memoryObjectStore) Manifest(name, tag string) (*manifest.SignedManifest, error) {
	objStore.mutex.Lock()
	defer objStore.mutex.Unlock()

	manifest, ok := objStore.manifestStorage[name+":"+tag]
	if !ok {
		return nil, fmt.Errorf("No manifest found with Name: %q, Tag: %q", name, tag)
	}
	return manifest, nil
}

// 写入 manifest
func (objStore *memoryObjectStore) WriteManifest(name, tag string, manifest *manifest.SignedManifest) error {
	objStore.mutex.Lock()
	defer objStore.mutex.Unlock()

	objStore.manifestStorage[name+":"+tag] = manifest
	return nil
}

// 返回 layer. 不存在时则通过 memoryLayer 取得它
func (objStore *memoryObjectStore) Layer(dgst digest.Digest) (Layer, error) {
	objStore.mutex.Lock()
	defer objStore.mutex.Unlock()

	layer, ok := objStore.layerStorage[dgst]
	if !ok {
		layer = &memoryLayer{cond: sync.NewCond(new(sync.Mutex))}
		objStore.layerStorage[dgst] = layer
	}

	return layer, nil
}

// 内存中 layer
type memoryLayer struct {
	cond         *sync.Cond
	contents     []byte
	expectedSize int
	writing      bool
}

// 在内存中读取 layer
func (ml *memoryLayer) Reader() (LayerReader, error) {
	ml.cond.L.Lock()
	defer ml.cond.L.Unlock()
	
	// 不存在
	if ml.contents == nil {
		return nil, fmt.Errorf("Layer has not been written to yet")
	}
	// 正被写入
	if ml.writing {
		return nil, ErrLayerLocked
	}

	return &memoryLayerReader{ml: ml, reader: bytes.NewReader(ml.contents)}, nil
}

// 将 layer 写入内存
func (ml *memoryLayer) Writer() (LayerWriter, error) {
	ml.cond.L.Lock()
	defer ml.cond.L.Unlock()

	if ml.contents != nil {
		// 正被写入
		if ml.writing {
			return nil, ErrLayerLocked
		}
		// 已存在
		if ml.expectedSize == len(ml.contents) {
			return nil, ErrLayerAlreadyExists
		}
	} else {
		ml.contents = make([]byte, 0)
	}

	ml.writing = true
	return &memoryLayerWriter{ml: ml, buffer: bytes.NewBuffer(ml.contents)}, nil
}

// 等待写入完成
func (ml *memoryLayer) Wait() error {
	ml.cond.L.Lock()
	defer ml.cond.L.Unlock()

	if ml.contents == nil {
		return fmt.Errorf("No writer to wait on")
	}

	for ml.writing {
		ml.cond.Wait()
	}

	return nil
}

// 对内存 layer 只读
type memoryLayerReader struct {
	ml     *memoryLayer
	reader *bytes.Reader
}

// 读取
func (mlr *memoryLayerReader) Read(p []byte) (int, error) {
	return mlr.reader.Read(p)
}

// 关闭
func (mlr *memoryLayerReader) Close() error {
	return nil
}

// 当前大小
func (mlr *memoryLayerReader) CurrentSize() int {
	return len(mlr.ml.contents)
}

// 总大小
func (mlr *memoryLayerReader) Size() int {
	return mlr.ml.expectedSize
}

// 对内存 layer 只写
type memoryLayerWriter struct {
	ml     *memoryLayer
	buffer *bytes.Buffer
}

// 写入
func (mlw *memoryLayerWriter) Write(p []byte) (int, error) {
	if mlw.ml.expectedSize == 0 {
		return 0, fmt.Errorf("Must set size before writing to layer")
	}
	wrote, err := mlw.buffer.Write(p)
	mlw.ml.contents = mlw.buffer.Bytes()
	return wrote, err
}

// 关闭
func (mlw *memoryLayerWriter) Close() error {
	mlw.ml.cond.L.Lock()
	defer mlw.ml.cond.L.Unlock()

	return mlw.close()
}

// 关闭
func (mlw *memoryLayerWriter) close() error {
	mlw.ml.writing = false
	mlw.ml.cond.Broadcast()
	return nil
}

// 当前大小
func (mlw *memoryLayerWriter) CurrentSize() int {
	return len(mlw.ml.contents)
}

// 总大小
func (mlw *memoryLayerWriter) Size() int {
	return mlw.ml.expectedSize
}

// 设置大小
func (mlw *memoryLayerWriter) SetSize(size int) error {
	if !mlw.ml.writing {
		return fmt.Errorf("Layer is closed for writing")
	}
	mlw.ml.expectedSize = size
	return nil
}
