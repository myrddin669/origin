package etcd

import (
	"errors"
	"strconv"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	kubeetcd "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/image/api"
)

const (
	// ImagePath is the path to deployment image in etcd
	ImagePath string = "/images"
	// ImageRepositoriesPath is the path to imageRepository resources in etcd
	ImageRepositoriesPath string = "/imageRepositories"
)

// Etcd implements ImageRegistry and ImageRepositoryRegistry backed by etcd.
type Etcd struct {
	tools.EtcdHelper
}

// New returns a new etcd registry.
func New(helper tools.EtcdHelper) *Etcd {
	return &Etcd{
		EtcdHelper: helper,
	}
}

// ListImages retrieves a list of images that match selector.
func (r *Etcd) ListImages(ctx kubeapi.Context, selector labels.Selector) (*api.ImageList, error) {
	list := api.ImageList{}
	err := r.ExtractToList(makeImageListKey(ctx), &list)
	if err != nil {
		return nil, err
	}
	filtered := []api.Image{}
	for _, item := range list.Items {
		if selector.Matches(labels.Set(item.Labels)) {
			filtered = append(filtered, item)
		}
	}
	list.Items = filtered
	return &list, nil
}

func makeImageListKey(ctx kubeapi.Context) string {
	return kubeetcd.MakeEtcdListKey(ctx, ImagePath)
}

func makeImageKey(ctx kubeapi.Context, id string) (string, error) {
	return kubeetcd.MakeEtcdItemKey(ctx, ImagePath, id)
}

// GetImage retrieves a specific image
func (r *Etcd) GetImage(ctx kubeapi.Context, id string) (*api.Image, error) {
	var image api.Image
	key, err := makeImageKey(ctx, id)
	if err != nil {
		return nil, err
	}

	if err = r.ExtractObj(key, &image, false); err != nil {
		return nil, etcderr.InterpretGetError(err, "image", id)
	}
	return &image, nil
}

// CreateImage creates a new image
func (r *Etcd) CreateImage(ctx kubeapi.Context, image *api.Image) error {
	key, err := makeImageKey(ctx, image.ID)
	if err != nil {
		return err
	}

	err = r.CreateObj(key, image, 0)
	return etcderr.InterpretCreateError(err, "image", image.ID)
}

// UpdateImage updates an existing image
func (r *Etcd) UpdateImage(ctx kubeapi.Context, image *api.Image) error {
	return errors.New("not supported")
}

// DeleteImage deletes an existing image
func (r *Etcd) DeleteImage(ctx kubeapi.Context, id string) error {
	key, err := makeImageKey(ctx, id)
	if err != nil {
		return err
	}

	err = r.Delete(key, false)
	return etcderr.InterpretDeleteError(err, "image", id)
}

// ListImageRepositories retrieves a list of ImageRepositories that match selector.
func (r *Etcd) ListImageRepositories(ctx kubeapi.Context, selector labels.Selector) (*api.ImageRepositoryList, error) {
	list := api.ImageRepositoryList{}
	err := r.ExtractToList(makeImageRepositoryListKey(ctx), &list)
	if err != nil {
		return nil, err
	}
	filtered := []api.ImageRepository{}
	for _, item := range list.Items {
		if selector.Matches(labels.Set(item.Labels)) {
			filtered = append(filtered, item)
		}
	}
	list.Items = filtered
	return &list, nil
}

func makeImageRepositoryListKey(ctx kubeapi.Context) string {
	return kubeetcd.MakeEtcdListKey(ctx, ImageRepositoriesPath)
}

func makeImageRepositoryKey(ctx kubeapi.Context, id string) (string, error) {
	return kubeetcd.MakeEtcdItemKey(ctx, ImageRepositoriesPath, id)
}

// GetImageRepository retrieves an ImageRepository by id.
func (r *Etcd) GetImageRepository(ctx kubeapi.Context, id string) (*api.ImageRepository, error) {
	var repo api.ImageRepository
	key, err := makeImageRepositoryKey(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = r.ExtractObj(key, &repo, false); err != nil {
		return nil, etcderr.InterpretGetError(err, "imageRepository", id)
	}
	return &repo, nil
}

// TODO expose this from kubernetes.  I will do that, but I don't want this merge stuck on kubernetes refactoring
// parseWatchResourceVersion takes a resource version argument and converts it to
// the etcd version we should pass to helper.Watch(). Because resourceVersion is
// an opaque value, the default watch behavior for non-zero watch is to watch
// the next value (if you pass "1", you will see updates from "2" onwards).
func parseWatchResourceVersion(resourceVersion, kind string) (uint64, error) {
	if resourceVersion == "" || resourceVersion == "0" {
		return 0, nil
	}
	version, err := strconv.ParseUint(resourceVersion, 10, 64)
	if err != nil {
		return 0, etcderr.InterpretResourceVersionError(err, kind, resourceVersion)
	}
	return version + 1, nil
}

// WatchImageRepositories begins watching for new, changed, or deleted ImageRepositories.
func (r *Etcd) WatchImageRepositories(ctx kubeapi.Context, resourceVersion string, filter func(repo *api.ImageRepository) bool) (watch.Interface, error) {
	version, err := parseWatchResourceVersion(resourceVersion, "imageRepository")
	if err != nil {
		return nil, err
	}

	return r.WatchList(makeImageRepositoryListKey(ctx), version, func(obj runtime.Object) bool {
		repo, ok := obj.(*api.ImageRepository)
		if !ok {
			glog.Errorf("Unexpected object during image repository watch: %#v", obj)
			return false
		}
		return filter(repo)
	})
}

// CreateImageRepository registers the given ImageRepository.
func (r *Etcd) CreateImageRepository(ctx kubeapi.Context, repo *api.ImageRepository) error {
	key, err := makeImageRepositoryKey(ctx, repo.ID)
	if err != nil {
		return err
	}
	err = r.CreateObj(key, repo, 0)
	return etcderr.InterpretCreateError(err, "imageRepository", repo.ID)
}

// UpdateImageRepository replaces an existing ImageRepository in the registry with the given ImageRepository.
func (r *Etcd) UpdateImageRepository(ctx kubeapi.Context, repo *api.ImageRepository) error {
	key, err := makeImageRepositoryKey(ctx, repo.ID)
	if err != nil {
		return err
	}
	err = r.SetObj(key, repo)
	return etcderr.InterpretUpdateError(err, "imageRepository", repo.ID)
}

// DeleteImageRepository deletes an ImageRepository by id.
func (r *Etcd) DeleteImageRepository(ctx kubeapi.Context, id string) error {
	key, err := makeImageRepositoryKey(ctx, id)
	if err != nil {
		return err
	}
	err = r.Delete(key, false)
	return etcderr.InterpretDeleteError(err, "imageRepository", id)
}
