package distribution

import (
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
)

// Scope defines the set of items that match a namespace.
type Scope interface {
	// Contains returns true if the name belongs to the namespace.
	Contains(name string) bool
}

type fullScope struct{}

func (f fullScope) Contains(string) bool {
	return true
}

// GlobalScope represents the full namespace scope which contains
// all other scopes.
// 全局 scope
var GlobalScope = Scope(fullScope{})

// Namespace represents a collection of repositories, addressable by name.
// Generally, a namespace is backed by a set of one or more services,
// providing facilities such as registry access, trust, and indexing.
type Namespace interface {
	// Scope describes the names that can be used with this Namespace. The
	// global namespace will have a scope that matches all names. The scope
	// effectively provides an identity for the namespace.
	// 描述可以被 namespace 使用的名字
	Scope() Scope

	// Repository should return a reference to the named repository. The
	// registry may or may not have the repository but should always return a
	// reference.
	// 一定会返回一个命名的 registry 的引用 ?
	Repository(ctx context.Context, name string) (Repository, error)
}

// Repository is a named collection of manifests and layers.
// manifest 和 layers 的库
type Repository interface {
	// Name returns the name of the repository.
	// 名字
	Name() string

	// Manifests returns a reference to this repository's manifest service.
	Manifests() ManifestService

	// Blobs returns a reference to this repository's blob service.
	Blobs(ctx context.Context) BlobStore

	// TODO(stevvooe): The above BlobStore return can probably be relaxed to
	// be a BlobService for use with clients. This will allow such
	// implementations to avoid implementing ServeBlob.

	// Signatures returns a reference to this repository's signatures service.
	Signatures() SignatureService
}

// TODO(stevvooe): Must add close methods to all these. May want to change the
// way instances are created to better reflect internal dependency
// relationships.

// ManifestService provides operations on image manifests.
// ManifestService 提供对 image manifests 的操作
type ManifestService interface {
	// Exists returns true if the manifest exists.
	// manifest 是否存在
	Exists(dgst digest.Digest) (bool, error)

	// Get retrieves the identified by the digest, if it exists.
	// 通过 digest 获取 manifest
	Get(dgst digest.Digest) (*manifest.SignedManifest, error)

	// Delete removes the manifest, if it exists.
	// 删除 manifest 不支持操作
	Delete(dgst digest.Digest) error

	// Put creates or updates the manifest.
	// 创建或者更新一个 manifest
	Put(manifest *manifest.SignedManifest) error

	// TODO(stevvooe): The methods after this message should be moved to a
	// discrete TagService, per active proposals.

	// Tags lists the tags under the named repository.
	// 列出 repository 的 tag
	Tags() ([]string, error)

	// ExistsByTag returns true if the manifest exists.
	// 通过 tag 判断 manifest 是否存在
	ExistsByTag(tag string) (bool, error)

	// GetByTag retrieves the named manifest, if it exists.
	// 通过 tag 获得 manifest
	GetByTag(tag string) (*manifest.SignedManifest, error)

	// TODO(stevvooe): There are several changes that need to be done to this
	// interface:
	//
	//	1. Allow explicit tagging with Tag(digest digest.Digest, tag string)
	//	2. Support reading tags with a re-entrant reader to avoid large
	//       allocations in the registry.
	//	3. Long-term: Provide All() method that lets one scroll through all of
	//       the manifest entries.
	//	4. Long-term: break out concept of signing from manifests. This is
	//       really a part of the distribution sprint.
	//	5. Long-term: Manifest should be an interface. This code shouldn't
	//       really be concerned with the storage format.
}

// SignatureService provides operations on signatures.
type SignatureService interface {
	// Get retrieves all of the signature blobs for the specified digest.
	Get(dgst digest.Digest) ([][]byte, error)

	// Put stores the signature for the provided digest.
	Put(dgst digest.Digest, signatures ...[]byte) error
}
