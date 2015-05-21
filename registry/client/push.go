package client

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution/manifest"
)

// simultaneousLayerPushWindow is the size of the parallel layer push window.
// A layer may not be pushed until the layer preceeding it by the length of the
// push window has been successfully pushed.
// 最大流动窗口
const simultaneousLayerPushWindow = 4

type pushFunction func(fsLayer manifest.FSLayer) error

// Push implements a client push workflow for the image defined by the given
// name and tag pair, using the given ObjectStore for local manifest and layer
// storage
// push 流程
func Push(c Client, objectStore ObjectStore, name, tag string) error {
	// 获得 manifest
	manifest, err := objectStore.Manifest(name, tag)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"name":  name,
			"tag":   tag,
		}).Info("No image found")
		return err
	}
	
	// 给每一个 layer 建立 channel
	errChans := make([]chan error, len(manifest.FSLayers))
	for i := range manifest.FSLayers {
		errChans[i] = make(chan error)
	}
	
	// 取消的 channel
	cancelCh := make(chan struct{})

	// Iterate over each layer in the manifest, simultaneously pushing no more
	// than simultaneousLayerPushWindow layers at a time. If an error is
	// received from a layer push, we abort the push.
	// 每个 layer 进行 push
	for i := 0; i < len(manifest.FSLayers)+simultaneousLayerPushWindow; i++ {
		dependentLayer := i - simultaneousLayerPushWindow
		if dependentLayer >= 0 {
			err := <-errChans[dependentLayer]
			if err != nil {
				log.WithField("error", err).Warn("Push aborted")
				close(cancelCh)
				return err
			}
		}

		if i < len(manifest.FSLayers) {
			go func(i int) {
				// push 成功或是取消
				select {
				case errChans[i] <- pushLayer(c, objectStore, name, manifest.FSLayers[i]):
				case <-cancelCh: // recv broadcast notification about cancelation
				}
			}(i)
		}
	}
	
	// 写 iamges manifest ?
	err = c.PutImageManifest(name, tag, manifest)
	if err != nil {
		log.WithFields(log.Fields{
			"error":    err,
			"manifest": manifest,
		}).Warn("Unable to upload manifest")
		return err
	}

	return nil
}

// push 一个 layer
func pushLayer(c Client, objectStore ObjectStore, name string, fsLayer manifest.FSLayer) error {
	log.WithField("layer", fsLayer).Info("Pushing layer")
	
	// 取得一个 layer
	layer, err := objectStore.Layer(fsLayer.BlobSum)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to read local layer")
		return err
	}
	
	// 建立一个 layer 的 reader
	layerReader, err := layer.Reader()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to read local layer")
		return err
	}
	defer layerReader.Close()
	
	// 读取到的 layer 不全， 不能 push
	if layerReader.CurrentSize() != layerReader.Size() {
		log.WithFields(log.Fields{
			"layer":       fsLayer,
			"currentSize": layerReader.CurrentSize(),
			"size":        layerReader.Size(),
		}).Warn("Local layer incomplete")
		return fmt.Errorf("Local layer incomplete")
	}
	
	// 取得二进制对象长度
	length, err := c.BlobLength(name, fsLayer.BlobSum)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to check existence of remote layer")
		return err
	}
	// 大于 0 说明之前上传过
	if length >= 0 {
		log.WithField("layer", fsLayer).Info("Layer already exists")
		return nil
	}
	
	// 初始化二进制文件上传
	location, err := c.InitiateBlobUpload(name)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to upload layer")
		return err
	}
	
	// 开始上传二进制文件
	err = c.UploadBlob(location, layerReader, int(layerReader.CurrentSize()), fsLayer.BlobSum)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to upload layer")
		return err
	}

	return nil
}
