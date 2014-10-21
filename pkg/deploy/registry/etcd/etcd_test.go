package etcd

import (
	"fmt"
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/coreos/go-etcd/etcd"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/deploy/api"
)

// This copy and paste is not pure ignorance.  This is that we can be sure that the key is getting made as we
// expect it to. If someone changes the location of these resources by say moving all the resources to
// "/origin/resources" (which is a really good idea), then they've made a breaking change and something should
// fail to let them know they've change some significant change and that other dependent pieces may break.
func makeTestDeploymentListKey(namespace string) string {
	if len(namespace) != 0 {
		return "/deployments/" + namespace
	}
	return "/deployments"
}
func makeTestDeploymentKey(namespace, id string) string {
	return "/deployments/" + namespace + "/" + id
}
func makeTestDefaultDeploymentKey(id string) string {
	return makeTestDeploymentKey(kubeapi.NamespaceDefault, id)
}
func makeTestDefaultDeploymentListKey() string {
	return makeTestDeploymentListKey(kubeapi.NamespaceDefault)
}
func makeTestDeploymentConfigListKey(namespace string) string {
	if len(namespace) != 0 {
		return "/deploymentConfigs/" + namespace
	}
	return "/deploymentConfigs"
}
func makeTestDeploymentConfigKey(namespace, id string) string {
	return "/deploymentConfigs/" + namespace + "/" + id
}
func makeTestDefaultDeploymentConfigKey(id string) string {
	return makeTestDeploymentConfigKey(kubeapi.NamespaceDefault, id)
}
func makeTestDefaultDeploymentConfigListKey() string {
	return makeTestDeploymentConfigListKey(kubeapi.NamespaceDefault)
}

func NewTestEtcd(client tools.EtcdClient) *Etcd {
	return New(tools.EtcdHelper{client, latest.Codec, tools.RuntimeVersionAdapter{latest.ResourceVersioner}})
}

func TestEtcdListEmptyDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultDeploymentListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	deployments, err := registry.ListDeployments(kubeapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deployments.Items) != 0 {
		t.Errorf("Unexpected deployments list: %#v", deployments)
	}
}

func TestEtcdListErrorDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultDeploymentListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: fmt.Errorf("some error"),
	}
	registry := NewTestEtcd(fakeClient)
	deployments, err := registry.ListDeployments(kubeapi.NewDefaultContext(), labels.Everything())
	if err == nil {
		t.Error("unexpected nil error")
	}

	if deployments != nil {
		t.Errorf("Unexpected non-nil deployments: %#v", deployments)
	}
}

func TestEtcdListEverythingDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultDeploymentListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Deployment{TypeMeta: kubeapi.TypeMeta{ID: "foo"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Deployment{TypeMeta: kubeapi.TypeMeta{ID: "bar"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	deployments, err := registry.ListDeployments(kubeapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deployments.Items) != 2 || deployments.Items[0].ID != "foo" || deployments.Items[1].ID != "bar" {
		t.Errorf("Unexpected deployments list: %#v", deployments)
	}
}

func TestEtcdListFilteredDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultDeploymentListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Deployment{
							TypeMeta: kubeapi.TypeMeta{ID: "foo"},
							Labels:   map[string]string{"env": "prod"},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Deployment{
							TypeMeta: kubeapi.TypeMeta{ID: "bar"},
							Labels:   map[string]string{"env": "dev"},
						}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	deployments, err := registry.ListDeployments(kubeapi.NewDefaultContext(), labels.SelectorFromSet(labels.Set{"env": "dev"}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deployments.Items) != 1 || deployments.Items[0].ID != "bar" {
		t.Errorf("Unexpected deployments list: %#v", deployments)
	}
}

func TestEtcdGetDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set(makeTestDefaultDeploymentKey("foo"), runtime.EncodeOrDie(latest.Codec, &api.Deployment{TypeMeta: kubeapi.TypeMeta{ID: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	deployment, err := registry.GetDeployment(kubeapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if deployment.ID != "foo" {
		t.Errorf("Unexpected deployment: %#v", deployment)
	}
}

func TestEtcdGetNotFoundDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultDeploymentKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	deployment, err := registry.GetDeployment(kubeapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
	if deployment != nil {
		t.Errorf("Unexpected deployment: %#v", deployment)
	}
}

func TestEtcdCreateDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data[makeTestDefaultDeploymentKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateDeployment(kubeapi.NewDefaultContext(), &api.Deployment{
		TypeMeta: kubeapi.TypeMeta{
			ID: "foo",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get(makeTestDefaultDeploymentKey("foo"), false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var deployment api.Deployment
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &deployment)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if deployment.ID != "foo" {
		t.Errorf("Unexpected deployment: %#v %s", deployment, resp.Node.Value)
	}
}

func TestEtcdCreateAlreadyExistsDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultDeploymentKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.Deployment{TypeMeta: kubeapi.TypeMeta{ID: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateDeployment(kubeapi.NewDefaultContext(), &api.Deployment{
		TypeMeta: kubeapi.TypeMeta{
			ID: "foo",
		},
	})
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsAlreadyExists(err) {
		t.Errorf("Expected 'already exists' error, got %#v", err)
	}
}

func TestEtcdUpdateOkDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	err := registry.UpdateDeployment(kubeapi.NewDefaultContext(), &api.Deployment{TypeMeta: kubeapi.TypeMeta{ID: "foo"}})
	if err != nil {
		t.Error("Unexpected error: %#v", err)
	}
}

func TestEtcdDeleteNotFoundDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = tools.EtcdErrorNotFound
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteDeployment(kubeapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("Expected 'not found' error, got %#v", err)
	}
}

func TestEtcdDeleteErrorDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = fmt.Errorf("Some error")
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteDeployment(kubeapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteOkDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	key := makeTestDefaultDeploymentListKey() + "/foo"

	err := registry.DeleteDeployment(kubeapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if len(fakeClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeClient.DeletedKeys)
	} else if fakeClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeClient.DeletedKeys[0], key)
	}
}

func TestEtcdListEmptyDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultDeploymentConfigListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	deploymentConfigs, err := registry.ListDeploymentConfigs(kubeapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deploymentConfigs.Items) != 0 {
		t.Errorf("Unexpected deploymentConfigs list: %#v", deploymentConfigs)
	}
}

func TestEtcdListErrorDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultDeploymentConfigListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: fmt.Errorf("some error"),
	}
	registry := NewTestEtcd(fakeClient)
	deploymentConfigs, err := registry.ListDeploymentConfigs(kubeapi.NewDefaultContext(), labels.Everything())
	if err == nil {
		t.Error("unexpected nil error")
	}

	if deploymentConfigs != nil {
		t.Errorf("Unexpected non-nil deploymentConfigs: %#v", deploymentConfigs)
	}
}

func TestEtcdListEverythingDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultDeploymentConfigListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{TypeMeta: kubeapi.TypeMeta{ID: "foo"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{TypeMeta: kubeapi.TypeMeta{ID: "bar"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	deploymentConfigs, err := registry.ListDeploymentConfigs(kubeapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deploymentConfigs.Items) != 2 || deploymentConfigs.Items[0].ID != "foo" || deploymentConfigs.Items[1].ID != "bar" {
		t.Errorf("Unexpected deploymentConfigs list: %#v", deploymentConfigs)
	}
}

func TestEtcdListFilteredDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultDeploymentConfigListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{
							TypeMeta: kubeapi.TypeMeta{ID: "foo"},
							Labels:   map[string]string{"env": "prod"},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{
							TypeMeta: kubeapi.TypeMeta{ID: "bar"},
							Labels:   map[string]string{"env": "dev"},
						}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	deploymentConfigs, err := registry.ListDeploymentConfigs(kubeapi.NewDefaultContext(), labels.SelectorFromSet(labels.Set{"env": "dev"}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deploymentConfigs.Items) != 1 || deploymentConfigs.Items[0].ID != "bar" {
		t.Errorf("Unexpected deploymentConfigs list: %#v", deploymentConfigs)
	}
}

func TestEtcdGetDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set(makeTestDefaultDeploymentConfigKey("foo"), runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{TypeMeta: kubeapi.TypeMeta{ID: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	deployment, err := registry.GetDeploymentConfig(kubeapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if deployment.ID != "foo" {
		t.Errorf("Unexpected deployment: %#v", deployment)
	}
}

func TestEtcdGetNotFoundDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultDeploymentConfigKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	deployment, err := registry.GetDeploymentConfig(kubeapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
	if deployment != nil {
		t.Errorf("Unexpected deployment: %#v", deployment)
	}
}

func TestEtcdCreateDeploymentConfig(t *testing.T) {
	key := makeTestDefaultDeploymentConfigKey("foo")
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateDeploymentConfig(kubeapi.NewDefaultContext(), &api.DeploymentConfig{
		TypeMeta: kubeapi.TypeMeta{
			ID: "foo",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get(key, false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var d api.DeploymentConfig
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &d)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if d.ID != "foo" {
		t.Errorf("Unexpected deploymentConfig: %#v %s", d, resp.Node.Value)
	}
}

func TestEtcdCreateAlreadyExistsDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultDeploymentConfigKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{TypeMeta: kubeapi.TypeMeta{ID: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateDeploymentConfig(kubeapi.NewDefaultContext(), &api.DeploymentConfig{
		TypeMeta: kubeapi.TypeMeta{
			ID: "foo",
		},
	})
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsAlreadyExists(err) {
		t.Errorf("Expected 'already exists' error, got %#v", err)
	}
}

func TestEtcdUpdateOkDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	err := registry.UpdateDeploymentConfig(kubeapi.NewDefaultContext(), &api.DeploymentConfig{TypeMeta: kubeapi.TypeMeta{ID: "foo"}})
	if err != nil {
		t.Error("Unexpected error %#v", err)
	}
}

func TestEtcdDeleteNotFoundDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = tools.EtcdErrorNotFound
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteDeploymentConfig(kubeapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("Expected 'not found' error, got %#v", err)
	}
}

func TestEtcdDeleteErrorDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = fmt.Errorf("Some error")
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteDeploymentConfig(kubeapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteOkDeploymentConfig(t *testing.T) {
	key := makeTestDefaultDeploymentConfigKey("foo")
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteDeploymentConfig(kubeapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if len(fakeClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeClient.DeletedKeys)
	} else if fakeClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeClient.DeletedKeys[0], key)
	}
}

func TestEtcdCreateDeploymentConfigFailsWithoutNamespace(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateDeploymentConfig(kubeapi.NewContext(), &api.DeploymentConfig{
		TypeMeta: kubeapi.TypeMeta{
			ID: "foo",
		},
	})

	if err == nil {
		t.Errorf("expected error that namespace was missing from context")
	}
}

func TestEtcdCreateDeploymentFailsWithoutNamespace(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateDeployment(kubeapi.NewContext(), &api.Deployment{
		TypeMeta: kubeapi.TypeMeta{
			ID: "foo",
		},
	})

	if err == nil {
		t.Errorf("expected error that namespace was missing from context")
	}
}

func TestEtcdListDeploymentsInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kubeapi.WithNamespace(kubeapi.NewContext(), "alfa")
	namespaceBravo := kubeapi.WithNamespace(kubeapi.NewContext(), "bravo")
	fakeClient.Data["/deployments/alfa"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Deployment{TypeMeta: kubeapi.TypeMeta{ID: "foo1"}}),
					},
				},
			},
		},
		E: nil,
	}
	fakeClient.Data["/deployments/bravo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Deployment{TypeMeta: kubeapi.TypeMeta{ID: "foo2"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Deployment{TypeMeta: kubeapi.TypeMeta{ID: "bar2"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)

	deploymentsAlfa, err := registry.ListDeployments(namespaceAlfa, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(deploymentsAlfa.Items) != 1 || deploymentsAlfa.Items[0].ID != "foo1" {
		t.Errorf("Unexpected deployments list: %#v", deploymentsAlfa)
	}

	deploymentsBravo, err := registry.ListDeployments(namespaceBravo, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(deploymentsBravo.Items) != 2 || deploymentsBravo.Items[0].ID != "foo2" || deploymentsBravo.Items[1].ID != "bar2" {
		t.Errorf("Unexpected deployments list: %#v", deploymentsBravo)
	}
}

func TestEtcdListDeploymentConfigsInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kubeapi.WithNamespace(kubeapi.NewContext(), "alfa")
	namespaceBravo := kubeapi.WithNamespace(kubeapi.NewContext(), "bravo")
	fakeClient.Data["/deploymentConfigs/alfa"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{TypeMeta: kubeapi.TypeMeta{ID: "foo1"}}),
					},
				},
			},
		},
		E: nil,
	}
	fakeClient.Data["/deploymentConfigs/bravo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{TypeMeta: kubeapi.TypeMeta{ID: "foo2"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{TypeMeta: kubeapi.TypeMeta{ID: "bar2"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)

	deploymentConfigsAlfa, err := registry.ListDeploymentConfigs(namespaceAlfa, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(deploymentConfigsAlfa.Items) != 1 || deploymentConfigsAlfa.Items[0].ID != "foo1" {
		t.Errorf("Unexpected deployments list: %#v", deploymentConfigsAlfa)
	}

	deploymentConfigsBravo, err := registry.ListDeploymentConfigs(namespaceBravo, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(deploymentConfigsBravo.Items) != 2 || deploymentConfigsBravo.Items[0].ID != "foo2" || deploymentConfigsBravo.Items[1].ID != "bar2" {
		t.Errorf("Unexpected deployments list: %#v", deploymentConfigsBravo)
	}
}

func TestEtcdGetDeploymentConfigInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kubeapi.WithNamespace(kubeapi.NewContext(), "alfa")
	namespaceBravo := kubeapi.WithNamespace(kubeapi.NewContext(), "bravo")
	fakeClient.Set("/deploymentConfigs/alfa/foo", runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{TypeMeta: kubeapi.TypeMeta{ID: "foo"}}), 0)
	fakeClient.Set("/deploymentConfigs/bravo/foo", runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{TypeMeta: kubeapi.TypeMeta{ID: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)

	alfaFoo, err := registry.GetDeploymentConfig(namespaceAlfa, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if alfaFoo == nil || alfaFoo.ID != "foo" {
		t.Errorf("Unexpected deployment: %#v", alfaFoo)
	}

	bravoFoo, err := registry.GetDeploymentConfig(namespaceBravo, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if bravoFoo == nil || bravoFoo.ID != "foo" {
		t.Errorf("Unexpected deployment: %#v", bravoFoo)
	}
}

func TestEtcdGetDeploymentInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kubeapi.WithNamespace(kubeapi.NewContext(), "alfa")
	namespaceBravo := kubeapi.WithNamespace(kubeapi.NewContext(), "bravo")
	fakeClient.Set("/deployments/alfa/foo", runtime.EncodeOrDie(latest.Codec, &api.Deployment{TypeMeta: kubeapi.TypeMeta{ID: "foo"}}), 0)
	fakeClient.Set("/deployments/bravo/foo", runtime.EncodeOrDie(latest.Codec, &api.Deployment{TypeMeta: kubeapi.TypeMeta{ID: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)

	alfaFoo, err := registry.GetDeployment(namespaceAlfa, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if alfaFoo == nil || alfaFoo.ID != "foo" {
		t.Errorf("Unexpected deployment: %#v", alfaFoo)
	}

	bravoFoo, err := registry.GetDeployment(namespaceBravo, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if bravoFoo == nil || bravoFoo.ID != "foo" {
		t.Errorf("Unexpected deployment: %#v", bravoFoo)
	}
}
