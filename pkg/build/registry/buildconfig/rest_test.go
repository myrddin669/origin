package buildconfig

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta1"
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/test"
)

func TestNewConfig(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := REST{&mockRegistry}
	obj := storage.New()
	_, ok := obj.(*api.BuildConfig)
	if !ok {
		t.Error("New did not return an object of type *BuildConfig")
	}
}

func TestGetConfig(t *testing.T) {
	expectedConfig := mockBuildConfig()
	mockRegistry := test.BuildConfigRegistry{BuildConfig: expectedConfig}
	storage := REST{&mockRegistry}
	configObj, err := storage.Get(kubeapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	config, ok := configObj.(*api.BuildConfig)
	if !ok {
		t.Errorf("A build config was not returned: %v", configObj)
	}
	if config.ID != expectedConfig.ID {
		t.Errorf("Unexpected build config returned: %v", config)
	}
}

func TestGetConfigError(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{Err: fmt.Errorf("get error")}
	storage := REST{&mockRegistry}
	buildObj, err := storage.Get(kubeapi.NewDefaultContext(), "foo")
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRegistry.Err, err)
	}
	if buildObj != nil {
		t.Errorf("Unexpected non-nil build: %#v", buildObj)
	}
}

func TestDeleteBuild(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	configId := "test-config-id"
	storage := REST{&mockRegistry}
	channel, err := storage.Delete(kubeapi.NewDefaultContext(), configId)
	if err != nil {
		t.Errorf("Unexpected error when deleting: %v", err)
	}
	select {
	case result := <-channel:
		status, ok := result.(*kubeapi.Status)
		if !ok {
			t.Errorf("Unexpected operation result: %v", result)
		}
		if status.Status != kubeapi.StatusSuccess {
			t.Errorf("Unexpected failure status: %v", status)
		}
		if mockRegistry.DeletedConfigId != configId {
			t.Errorf("Unexpected build id was deleted: %v", mockRegistry.DeletedConfigId)
		}
		// expected case
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestDeleteBuildError(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{Err: fmt.Errorf("Delete error")}
	configId := "test-config-id"
	storage := REST{&mockRegistry}
	channel, _ := storage.Delete(kubeapi.NewDefaultContext(), configId)
	select {
	case result := <-channel:
		status, ok := result.(*kubeapi.Status)
		if !ok {
			t.Errorf("Unexpected operation result: %#v", channel)
		}
		if status.Message != mockRegistry.Err.Error() {
			t.Errorf("Unexpected status returned: %#v", status)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestListConfigsError(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{
		Err: fmt.Errorf("test error"),
	}
	storage := REST{&mockRegistry}
	configs, err := storage.List(kubeapi.NewDefaultContext(), nil, nil)
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRegistry.Err, err)
	}
	if configs != nil {
		t.Errorf("Unexpected non-nil buildConfig list: %#v", configs)
	}
}

func TestListEmptyConfigList(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{BuildConfigs: &api.BuildConfigList{TypeMeta: kubeapi.TypeMeta{ResourceVersion: "1"}}}
	storage := REST{&mockRegistry}
	buildConfigs, err := storage.List(kubeapi.NewDefaultContext(), labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(buildConfigs.(*api.BuildConfigList).Items) != 0 {
		t.Errorf("Unexpected non-zero ctrl list: %#v", buildConfigs)
	}
	if buildConfigs.(*api.BuildConfigList).ResourceVersion != "1" {
		t.Errorf("Unexpected resource version: %#v", buildConfigs)
	}
}

func TestListConfigs(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{
		BuildConfigs: &api.BuildConfigList{
			Items: []api.BuildConfig{
				{
					TypeMeta: kubeapi.TypeMeta{
						ID: "foo",
					},
				},
				{
					TypeMeta: kubeapi.TypeMeta{
						ID: "bar",
					},
				},
			},
		},
	}
	storage := REST{&mockRegistry}
	configsObj, err := storage.List(kubeapi.NewDefaultContext(), labels.Everything(), labels.Everything())
	configs := configsObj.(*api.BuildConfigList)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(configs.Items) != 2 {
		t.Errorf("Unexpected buildConfig list: %#v", configs)
	}
	if configs.Items[0].ID != "foo" {
		t.Errorf("Unexpected buildConfig: %#v", configs.Items[0])
	}
	if configs.Items[1].ID != "bar" {
		t.Errorf("Unexpected buildConfig: %#v", configs.Items[1])
	}
}

func TestBuildConfigDecode(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := REST{registry: &mockRegistry}
	buildConfig := &api.BuildConfig{
		TypeMeta: kubeapi.TypeMeta{
			ID: "foo",
		},
	}
	body, err := latest.Codec.Encode(buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	buildConfigOut := storage.New()
	if err := latest.Codec.DecodeInto(body, buildConfigOut); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(buildConfig, buildConfigOut) {
		t.Errorf("Expected %#v, found %#v", buildConfig, buildConfigOut)
	}
}

func TestBuildConfigParsing(t *testing.T) {
	expectedBuildConfig := mockBuildConfig()
	file, err := ioutil.TempFile("", "buildConfig")
	fileName := file.Name()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	data, err := json.Marshal(expectedBuildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = file.Write(data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = file.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	data, err = ioutil.ReadFile(fileName)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	var buildConfig api.BuildConfig
	err = json.Unmarshal(data, &buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(&buildConfig, expectedBuildConfig) {
		t.Errorf("Parsing failed: %s\ngot: %#v\nexpected: %#v", string(data), &buildConfig, expectedBuildConfig)
	}
}

func TestCreateBuildConfig(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := REST{&mockRegistry}
	buildConfig := mockBuildConfig()
	channel, err := storage.Create(kubeapi.NewDefaultContext(), buildConfig)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case <-channel:
		// expected case
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func mockBuildConfig() *api.BuildConfig {
	return &api.BuildConfig{
		TypeMeta: kubeapi.TypeMeta{
			ID:        "dataBuild",
			Namespace: kubeapi.NamespaceDefault,
		},
		DesiredInput: api.BuildInput{
			SourceURI: "http://my.build.com/the/buildConfig/Dockerfile",
			ImageTag:  "repository/dataBuild",
		},
		Labels: map[string]string{
			"name": "dataBuild",
		},
	}
}

func TestUpdateBuildConfig(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := REST{&mockRegistry}
	buildConfig := mockBuildConfig()
	channel, err := storage.Update(kubeapi.NewDefaultContext(), buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	select {
	case result := <-channel:
		switch obj := result.(type) {
		case *kubeapi.Status:
			t.Errorf("Unexpected operation error: %v", obj)

		case *api.BuildConfig:
			if !reflect.DeepEqual(buildConfig, obj) {
				t.Errorf("Updated build does not match input build."+
					" Expected: %v, Got: %v", buildConfig, obj)
			}
		default:
			t.Errorf("Unexpected result type: %v", result)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestUpdateBuildConfigError(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{Err: fmt.Errorf("Update error")}
	storage := REST{&mockRegistry}
	buildConfig := mockBuildConfig()
	channel, err := storage.Update(kubeapi.NewDefaultContext(), buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	select {
	case result := <-channel:
		switch obj := result.(type) {
		case *kubeapi.Status:
			if obj.Message != mockRegistry.Err.Error() {
				t.Errorf("Unexpected error result: %v", obj)
			}
		default:
			t.Errorf("Unexpected result type: %v", result)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestBuildConfigRESTValidatesCreate(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := REST{&mockRegistry}
	failureCases := map[string]api.BuildConfig{
		"blank sourceURI": {
			TypeMeta: kubeapi.TypeMeta{ID: "abc"},
			DesiredInput: api.BuildInput{
				SourceURI: "",
				ImageTag:  "data/image",
				STIInput: &api.STIBuildInput{
					BuilderImage: "builder/image",
				},
			},
		},
		"blank ImageTag": {
			TypeMeta: kubeapi.TypeMeta{ID: "abc"},
			DesiredInput: api.BuildInput{
				SourceURI: "http://github.com/test/source",
				ImageTag:  "",
			},
		},
		"blank BuilderImage": {
			TypeMeta: kubeapi.TypeMeta{ID: "abc"},
			DesiredInput: api.BuildInput{
				SourceURI: "http://github.com/test/source",
				ImageTag:  "data/image",
				STIInput: &api.STIBuildInput{
					BuilderImage: "",
				},
			},
		},
	}
	for desc, failureCase := range failureCases {
		c, err := storage.Create(kubeapi.NewDefaultContext(), &failureCase)
		if c != nil {
			t.Errorf("%s: Expected nil channel", desc)
		}
		if !errors.IsInvalid(err) {
			t.Errorf("%s: Expected to get an invalid resource error, got %v", desc, err)
		}
	}
}

func TestBuildRESTValidatesUpdate(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := REST{&mockRegistry}
	failureCases := map[string]api.BuildConfig{
		"empty ID": {
			TypeMeta: kubeapi.TypeMeta{ID: ""},
			DesiredInput: api.BuildInput{
				SourceURI: "http://github.com/test/source",
				ImageTag:  "data/image",
			},
		},
		"blank sourceURI": {
			TypeMeta: kubeapi.TypeMeta{ID: "abc"},
			DesiredInput: api.BuildInput{
				SourceURI: "",
				ImageTag:  "data/image",
				STIInput: &api.STIBuildInput{
					BuilderImage: "builder/image",
				},
			},
		},
		"blank ImageTag": {
			TypeMeta: kubeapi.TypeMeta{ID: "abc"},
			DesiredInput: api.BuildInput{
				SourceURI: "http://github.com/test/source",
				ImageTag:  "",
			},
		},
		"blank BuilderImage on STIBuildType": {
			TypeMeta: kubeapi.TypeMeta{ID: "abc"},
			DesiredInput: api.BuildInput{
				SourceURI: "http://github.com/test/source",
				ImageTag:  "data/image",
				STIInput: &api.STIBuildInput{
					BuilderImage: "",
				},
			},
		},
	}
	for desc, failureCase := range failureCases {
		c, err := storage.Update(kubeapi.NewDefaultContext(), &failureCase)
		if c != nil {
			t.Errorf("%s: Expected nil channel", desc)
		}
		if !errors.IsInvalid(err) {
			t.Errorf("%s: Expected to get an invalid resource error, got %v", desc, err)
		}
	}
}

func TestCreateBuildConfigConflictingNamespace(t *testing.T) {
	storage := REST{}

	channel, err := storage.Create(kubeapi.WithNamespace(kubeapi.NewContext(), "legal-name"), &api.BuildConfig{
		TypeMeta: kubeapi.TypeMeta{ID: "foo", Namespace: "some-value"},
	})

	if channel != nil {
		t.Error("Expected a nil channel, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func TestUpdateBuildConfigConflictingNamespace(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := REST{&mockRegistry}

	buildConfig := mockBuildConfig()
	channel, err := storage.Update(kubeapi.WithNamespace(kubeapi.NewContext(), "legal-name"), buildConfig)

	if channel != nil {
		t.Error("Expected a nil channel, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func checkExpectedNamespaceError(t *testing.T, err error) {
	expectedError := "BuildConfig.Namespace does not match the provided context"
	if err == nil {
		t.Errorf("Expected '" + expectedError + "', but we didn't get one")
	} else {
		e, ok := err.(kubeclient.APIStatus)
		if !ok {
			t.Errorf("error was not a statusError: %v", err)
		}
		if e.Status().Code != http.StatusConflict {
			t.Errorf("Unexpected failure status: %v", e.Status())
		}
		if strings.Index(err.Error(), expectedError) == -1 {
			t.Errorf("Expected '"+expectedError+"' error, got '%v'", err.Error())
		}
	}

}
