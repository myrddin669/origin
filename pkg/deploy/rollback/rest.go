package rollback

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/api/validation"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// REST provides a rollback generation endpoint. Only the Create method is implemented.
type REST struct {
	generator GeneratorClient
	codec     runtime.Codec
}

// GeneratorClient defines a local interface to a rollback generator for testability.
type GeneratorClient interface {
	GenerateRollback(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error)
	GetDeployment(ctx kapi.Context, name string) (*kapi.ReplicationController, error)
	GetDeploymentConfig(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error)
}

// Client provides an implementation of Generator client
type Client struct {
	GRFn func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error)
	RCFn func(ctx kapi.Context, name string) (*kapi.ReplicationController, error)
	DCFn func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error)
}

func (c Client) GetDeploymentConfig(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
	return c.DCFn(ctx, name)
}
func (c Client) GetDeployment(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
	return c.RCFn(ctx, name)
}
func (c Client) GenerateRollback(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
	return c.GRFn(from, to, spec)
}

// NewREST safely creates a new REST.
func NewREST(generator GeneratorClient, codec runtime.Codec) apiserver.RESTStorage {
	return &REST{
		generator: generator,
		codec:     codec,
	}
}

func (s *REST) New() runtime.Object {
	return &deployapi.DeploymentConfigRollback{}
}

// Create generates a new DeploymentConfig representing a rollback.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	rollback, ok := obj.(*deployapi.DeploymentConfigRollback)
	if !ok {
		return nil, fmt.Errorf("not a rollback spec: %#v", obj)
	}

	if errs := validation.ValidateDeploymentConfigRollback(rollback); len(errs) > 0 {
		return nil, kerrors.NewInvalid("DeploymentConfigRollback", "", errs)
	}

	// Roll back "from" the current deployment "to" a target deployment

	// Find the target ("to") deployment and decode the DeploymentConfig
	targetDeployment, err := s.generator.GetDeployment(ctx, rollback.Spec.From.Name)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, newInvalidDeploymentError(rollback, "Deployment not found")
		}
		return nil, newInvalidDeploymentError(rollback, fmt.Sprintf("%v", err))
	}

	to, err := deployutil.DecodeDeploymentConfig(targetDeployment, s.codec)
	if err != nil {
		return nil, newInvalidDeploymentError(rollback,
			fmt.Sprintf("Couldn't decode deploymentConfig from deployment: %v", err))
	}

	// Find the current ("from") version of the target deploymentConfig
	from, err := s.generator.GetDeploymentConfig(ctx, to.Name)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, newInvalidDeploymentError(rollback,
				fmt.Sprintf("Couldn't find a current deploymentConfig %s/%s", targetDeployment.Namespace, to.Name))
		}
		return nil, newInvalidDeploymentError(rollback,
			fmt.Sprintf("Error finding current deploymentConfig %s/%s: %v", targetDeployment.Namespace, to.Name, err))
	}

	return s.generator.GenerateRollback(from, to, &rollback.Spec)
}

func newInvalidDeploymentError(rollback *deployapi.DeploymentConfigRollback, reason string) error {
	err := kerrors.NewFieldInvalid("spec.from.name", rollback.Spec.From.Name, reason)
	return kerrors.NewInvalid("DeploymentConfigRollback", "", kerrors.ValidationErrorList{err})
}
