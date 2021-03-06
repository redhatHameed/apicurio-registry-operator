package apicurioregistry

import (
	ar "github.com/Apicurio/apicurio-registry-operator/pkg/apis/apicur/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// A long-lived singleton container for shared components
type Context struct {
	// More static stuff
	scheme       *runtime.Scheme
	log          logr.Logger
	nativeClient client.Client
	c            controller.Controller

	controlFunctions []ControlFunction
	// Components
	configuration *Configuration
	kubeFactory   *KubeFactory
	ocpFactory    *OCPFactory
	patchers      *Patchers
	clients       *Clients

	// ===
	resourceCache ResourceCache
	envCache      EnvCache

	// ===
	requeue bool
}

// Create a new context when the operator is deployed, provide mostly static data
func NewContext(c controller.Controller, scheme *runtime.Scheme, log logr.Logger, client client.Client) *Context {
	self := &Context{
		c:            c,
		scheme:       scheme,
		log:          log,
		nativeClient: client,
		requeue:      false,
	}
	self.controlFunctions = *new([]ControlFunction)
	self.configuration = NewConfiguration(log)

	self.clients = NewClients(self)
	self.patchers = NewPatchers(self)

	self.kubeFactory = NewKubeFactory(self)
	self.ocpFactory = NewOCPFactory(self)

	self.resourceCache = NewResourceCache()
	self.envCache = NewEnvCache()

	return self
}

// Refresh context's state on each reconciliation loop execution,
// BEFORE CF execution
func (this *Context) Update(spec *ar.ApicurioRegistry) {
	this.configuration.Update(spec)

	specEntry := NewResourceCacheEntry(spec.Name, spec)
	this.resourceCache.Set(RC_KEY_SPEC, specEntry)
}

func (this *Context) AddControlFunction(cf ControlFunction) {
	this.controlFunctions = append(this.controlFunctions, cf)
}

// ===
// Getters

func (this *Context) GetControlFunctions() []ControlFunction {
	return this.controlFunctions
}

func (this *Context) GetLog() logr.Logger {
	return this.log
}

func (this *Context) GetClients() *Clients {
	return this.clients
}

func (this *Context) GetConfiguration() *Configuration {
	return this.configuration
}

func (this *Context) GetPatchers() *Patchers {
	return this.patchers
}

func (this *Context) GetController() controller.Controller {
	return this.c
}

func (this *Context) GetKubeFactory() *KubeFactory {
	return this.kubeFactory
}

func (this *Context) GetOCPFactory() *OCPFactory {
	return this.ocpFactory
}

func (this *Context) GetScheme() *runtime.Scheme {
	return this.scheme
}

// Do not use unless necessary, use `GetClients()`
func (this *Context) GetNativeClient() client.Client {
	return this.nativeClient
}

func (this *Context) GetResourceCache() ResourceCache {
	return this.resourceCache
}

func (this *Context) GetEnvCache() EnvCache {
	return this.envCache
}

func (this *Context) SetRequeue() {
	this.requeue = true
}

func (this *Context) GetAndResetRequeue() bool {
	res := this.requeue
	this.requeue = false
	return res
}
