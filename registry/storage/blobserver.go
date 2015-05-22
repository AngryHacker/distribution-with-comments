package storage

import (
	"fmt"
	"net/http"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/storage/driver"
)

// TODO(stevvooe): This should configurable in the future.
const blobCacheControlMaxAge = 365 * 24 * time.Hour

// blobServer simply serves blobs from a driver instance using a path function
// to identify paths and a descriptor service to fill in metadata.
// 与 storage driver 打交道
type blobServer struct {
	driver  driver.StorageDriver
	statter distribution.BlobStatter
	pathFn  func(dgst digest.Digest) (string, error)
}

func (bs *blobServer) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst digest.Digest) error {
	desc, err := bs.statter.Stat(ctx, dgst)
	if err != nil {
		return err
	}

	path, err := bs.pathFn(desc.Digest)
	if err != nil {
		return err
	}

	redirectURL, err := bs.driver.URLFor(ctx, path, map[string]interface{}{"method": r.Method})

	switch err {
	case nil:
		// Redirect to storage URL.
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	case driver.ErrUnsupportedMethod:
		// Fallback to serving the content directly.
		br, err := newFileReader(ctx, bs.driver, path, desc.Length)
		if err != nil {
			return err
		}
		defer br.Close()

		w.Header().Set("ETag", desc.Digest.String()) // If-None-Match handled by ServeContent
		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%.f", blobCacheControlMaxAge.Seconds()))

		if w.Header().Get("Docker-Content-Digest") == "" {
			w.Header().Set("Docker-Content-Digest", desc.Digest.String())
		}

		if w.Header().Get("Content-Type") == "" {
			// Set the content type if not already set.
			w.Header().Set("Content-Type", desc.MediaType)
		}

		if w.Header().Get("Content-Length") == "" {
			// Set the content length if not already set.
			w.Header().Set("Content-Length", fmt.Sprint(desc.Length))
		}
		
		// 将内容通过 http 写到 storage
		http.ServeContent(w, r, desc.Digest.String(), time.Time{}, br)
	}

	// Some unexpected error.
	return err
}
