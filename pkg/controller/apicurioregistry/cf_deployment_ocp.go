package apicurioregistry

import (
	ar "github.com/Apicurio/apicurio-registry-operator/pkg/apis/apicur/v1alpha1"
	ocp_apps "github.com/openshift/api/apps/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var _ ControlFunction = &DeploymentOcpCF{}

type DeploymentOcpCF struct {
	ctx            *Context
	isCached       bool
	deployments    []ocp_apps.DeploymentConfig
	deploymentName string
}

func NewDeploymentOcpCF(ctx *Context) ControlFunction {

	err := ctx.GetController().Watch(&source.Kind{Type: &ocp_apps.DeploymentConfig{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &ar.ApicurioRegistry{},
	})

	if err != nil {
		panic("Error creating watch.")
	}

	return &DeploymentOcpCF{
		ctx:            ctx,
		isCached:       false,
		deployments:    make([]ocp_apps.DeploymentConfig, 0),
		deploymentName: RC_EMPTY_NAME,
	}
}

func (this *DeploymentOcpCF) Describe() string {
	return "DeploymentOcpCF"
}

func (this *DeploymentOcpCF) Sense() {

	// Observation #1
	// Get cached Deployment
	deploymentEntry, deploymentExists := this.ctx.GetResourceCache().Get(RC_KEY_DEPLOYMENT_OCP)
	if deploymentExists {
		this.deploymentName = deploymentEntry.GetName()
	} else {
		this.deploymentName = RC_EMPTY_NAME
	}
	this.isCached = deploymentExists

	// Observation #2
	// Get deployment(s) we *should* track
	this.deployments = make([]ocp_apps.DeploymentConfig, 0)
	deployments, err := this.ctx.GetClients().OCP().GetDeployments(
		this.ctx.GetConfiguration().GetAppNamespace(),
		&meta.ListOptions{
			LabelSelector: "app=" + this.ctx.GetConfiguration().GetAppName(),
		})
	if err == nil {
		for _, deployment := range deployments.Items {
			if deployment.GetObjectMeta().GetDeletionTimestamp() == nil {
				this.deployments = append(this.deployments, deployment)
			}
		}
	}

	// Update the status
	this.ctx.GetConfiguration().SetConfig(CFG_STA_DEPLOYMENT_NAME, this.deploymentName)
}

func (this *DeploymentOcpCF) Compare() bool {
	// Condition #1
	// If we already have a deployment cached, skip
	return !this.isCached
}

func (this *DeploymentOcpCF) Respond() {
	// Response #1
	// We already know about a deployment (name), and it is in the list
	if this.deploymentName != RC_EMPTY_NAME {
		contains := false
		for _, val := range this.deployments {
			if val.Name == this.deploymentName {
				contains = true
				this.ctx.GetResourceCache().Set(RC_KEY_DEPLOYMENT_OCP, NewResourceCacheEntry(val.Name, &val))
				break
			}
		}
		if !contains {
			this.deploymentName = RC_EMPTY_NAME
		}
	}
	// Response #2
	// Can follow #1, but there must be a single deployment available
	if this.deploymentName == RC_EMPTY_NAME && len(this.deployments) == 1 {
		deployment := this.deployments[0]
		this.deploymentName = deployment.Name
		this.ctx.GetResourceCache().Set(RC_KEY_DEPLOYMENT_OCP, NewResourceCacheEntry(deployment.Name, &deployment))
	}
	// Response #3 (and #4)
	// If there is no deployment available (or there are more than 1), just create a new one
	if this.deploymentName == RC_EMPTY_NAME && len(this.deployments) != 1 {
		deployment := this.ctx.GetOCPFactory().CreateDeployment()
		// leave the creation itself to patcher+creator so other CFs can update
		this.ctx.GetResourceCache().Set(RC_KEY_DEPLOYMENT_OCP, NewResourceCacheEntry(RC_EMPTY_NAME, deployment))
	}
}
