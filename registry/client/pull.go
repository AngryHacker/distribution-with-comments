package client

import (
	"fmt"
	"io"

	log "github.com/Sirupsen/logrus"

	"github.com/docker/distribution/manifest"
)

// simultaneousLayerPullWindow is the size of the parallel layer pull window.
// A layer may not be pulled until the layer preceeding it by the length of the
// pull window has been successfully pulled.
// 类似 TCP 流动窗口
const simultaneousLayerPullWindow = 4

// Pull implements a client pull workflow for the image defined by the given
// name and tag pair, using the given ObjectStore for local manifest and layer
// storage
// pull 一个 images 的流程
func Pull(c Client, objectStore ObjectStore, name, tag string) error {
	// 获得 manifest
	manifest, err := c.GetImageManifest(name, tag)
	if err != nil {
		return err
	}
	log.WithField("manifest", manifest).Info("Pulled manifest")

	if len(manifest.FSLayers) != len(manifest.History) {
		return fmt.Errorf("Length of history not equal to number of layers")
	}
	if len(manifest.FSLayers) == 0 {
		return fmt.Errorf("Image has no layers")
	}
	
	// 为每一层 layer 建立一个 channel 
	errChans := make([]chan error, len(manifest.FSLayers))
	for i := range manifest.FSLayers {
		errChans[i] = make(chan error)
	}

	// To avoid leak of goroutines we must notify
	// pullLayer goroutines about a cancelation,
	// otherwise they will lock forever.
	cancelCh := make(chan struct{})

	// Iterate over each layer in the manifest, simultaneously pulling no more
	// than simultaneousLayerPullWindow layers at a time. If an error is
	// received from a layer pull, we abort the push.
	// 对每个 manifest 中的 layer pull, 每次不超过最大窗口大小
	for i := 0; i < len(manifest.FSLayers)+simultaneousLayerPullWindow; i++ {
		dependentLayer := i - simultaneousLayerPullWindow
		if dependentLayer >= 0 {
			err := <-errChans[dependentLayer]
			if err != nil {
				log.WithField("error", err).Warn("Pull aborted")
				close(cancelCh)
				return err
			}
		}

		if i < len(manifest.FSLayers) {
			go func(i int) {
				// 或者对 layer 进行 pull, 或者收到 cancelCh 的信号
				select {
				case errChans[i] <- pullLayer(c, objectStore, name, manifest.FSLayers[i]):
				case <-cancelCh: // no chance to recv until cancelCh's closed
				}
			}(i)
		}
	}
	
	// 写到 manifest ?
	err = objectStore.WriteManifest(name, tag, manifest)
	if err != nil {
		log.WithFields(log.Fields{
			"error":    err,
			"manifest": manifest,
		}).Warn("Unable to write image manifest")
		return err
	}

	return nil
}

// pull 一层 layer
// 如果内存中不存在， 则从远程获取 blob 到 reader 再由 writer 写入内存
func pullLayer(c Client, objectStore ObjectStore, name string, fsLayer manifest.FSLayer) error {
	log.WithField("layer", fsLayer).Info("Pulling layer")

	layer, err := objectStore.Layer(fsLayer.BlobSum)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to write local layer")
		return err
	}

	layerWriter, err := layer.Writer()
	// layer 已经存在, 无需下载
	if err == ErrLayerAlreadyExists {
		log.WithField("layer", fsLayer).Info("Layer already exists")
		return nil
	}
	
	// layer 正在下载中， 无需开始新的下载
	if err == ErrLayerLocked {
		log.WithField("layer", fsLayer).Info("Layer download in progress, waiting")
		layer.Wait()
		return nil
	}
	// 错误， 返回
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to write local layer")
		return err
	}
	defer layerWriter.Close()
	
	// 之前已部分下载
	if layerWriter.CurrentSize() > 0 {
		log.WithFields(log.Fields{
			"layer":       fsLayer,
			"currentSize": layerWriter.CurrentSize(),
			"size":        layerWriter.Size(),
		}).Info("Layer partially downloaded, resuming")
	}
	
	// 获得二进制对象
	layerReader, length, err := c.GetBlob(name, fsLayer.BlobSum, layerWriter.CurrentSize())
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to download layer")
		return err
	}
	defer layerReader.Close()
	
	// 改变 layer 的 currentSize
	layerWriter.SetSize(layerWriter.CurrentSize() + length)
	
	// 把 layReader 读取到的写到 layerWriter 里
	_, err = io.Copy(layerWriter, layerReader)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to download layer")
		return err
	}
	// 下载未完成
	if layerWriter.CurrentSize() != layerWriter.Size() {
		log.WithFields(log.Fields{
			"size":        layerWriter.Size(),
			"currentSize": layerWriter.CurrentSize(),
			"layer":       fsLayer,
		}).Warn("Layer invalid size")
		return fmt.Errorf(
			"Wrote incorrect number of bytes for layer %v. Expected %d, Wrote %d",
			fsLayer, layerWriter.Size(), layerWriter.CurrentSize(),
		)
	}
	return nil
}
