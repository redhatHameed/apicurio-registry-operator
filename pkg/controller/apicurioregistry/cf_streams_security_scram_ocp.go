package apicurioregistry

import (
	ar "github.com/Apicurio/apicurio-registry-operator/pkg/apis/apicur/v1alpha1"
	ocp_apps "github.com/openshift/api/apps/v1"
	core "k8s.io/api/core/v1"
)

var _ ControlFunction = &StreamsSecurityScramOcpCF{}

type StreamsSecurityScramOcpCF struct {
	ctx                          *Context
	persistence                  string
	bootstrapServers             string
	truststoreSecretName         string
	valid                        bool
	foundTruststoreSecretName    string
	deploymentExists             bool
	deploymentEntry              ResourceCacheEntry
	scramUser                    string
	scramPasswordSecretName      string
	scramMechanism               string
	foundScramUser               string
	foundScramPasswordSecretName string
	foundScramMechanism          string
	mechOk                       bool
}

func NewStreamsSecurityScramOcpCF(ctx *Context) ControlFunction {
	return &StreamsSecurityScramOcpCF{
		ctx:                          ctx,
		persistence:                  "",
		bootstrapServers:             "",
		truststoreSecretName:         "",
		valid:                        false,
		foundTruststoreSecretName:    "",
		scramUser:                    "",
		scramPasswordSecretName:      "",
		scramMechanism:               "",
		foundScramUser:               "",
		foundScramPasswordSecretName: "",
		foundScramMechanism:          "",
		mechOk:                       false,
	}
}

func (this *StreamsSecurityScramOcpCF) Describe() string {
	return "StreamsSecurityScramOcpCF"
}

func (this *StreamsSecurityScramOcpCF) Sense() {
	// Observation #1
	// Read the config values
	if specEntry, exists := this.ctx.GetResourceCache().Get(RC_KEY_SPEC); exists {
		spec := specEntry.GetValue().(*ar.ApicurioRegistry)
		this.persistence = spec.Spec.Configuration.Persistence
		this.bootstrapServers = spec.Spec.Configuration.Streams.BootstrapServers

		this.truststoreSecretName = spec.Spec.Configuration.Streams.Security.Scram.TruststoreSecretName
		this.scramUser = spec.Spec.Configuration.Streams.Security.Scram.User
		this.scramPasswordSecretName = spec.Spec.Configuration.Streams.Security.Scram.PasswordSecretName
		this.scramMechanism = spec.Spec.Configuration.Streams.Security.Scram.Mechanism
	}

	if this.scramMechanism == "" {
		this.scramMechanism = "SCRAM-SHA-512"
	}

	// Observation #2
	// Deployment exists
	this.foundTruststoreSecretName = ""

	deploymentEntry, deploymentExists := this.ctx.GetResourceCache().Get(RC_KEY_DEPLOYMENT_OCP)
	if deploymentExists {
		deployment := deploymentEntry.GetValue().(*ocp_apps.DeploymentConfig)
		for i, v := range deployment.Spec.Template.Spec.Volumes {
			if v.Name == SCRAM_TRUSTSTORE_SECRET_VOLUME_NAME {
				this.foundTruststoreSecretName = deployment.Spec.Template.Spec.Volumes[i].VolumeSource.Secret.SecretName
			}
		}
	}
	this.deploymentExists = deploymentExists
	this.deploymentEntry = deploymentEntry

	if entry, exists := this.ctx.GetEnvCache().Get(ENV_REGISTRY_STREAMS_SCRAM_USER); exists {
		this.foundScramUser = entry.GetValue().Value
	}
	if entry, exists := this.ctx.GetEnvCache().Get(ENV_REGISTRY_STREAMS_SCRAM_PASSWORD); exists {
		this.foundScramPasswordSecretName = entry.GetValue().ValueFrom.SecretKeyRef.Name
	}

	mechTopology := ""
	mechStorage := ""
	if entry, exists := this.ctx.GetEnvCache().Get(ENV_REGISTRY_STREAMS_TOPOLOGY_SASL_MECHANISM); exists {
		mechTopology = entry.GetValue().Value
	}
	if entry, exists := this.ctx.GetEnvCache().Get(ENV_REGISTRY_STREAMS_STORAGE_PRODUCER_SASL_MECHANISM); exists {
		mechStorage = entry.GetValue().Value
	}

	// Observation #3
	// Validate the config values
	this.valid = this.persistence == "streams" && this.bootstrapServers != "" &&
		this.truststoreSecretName != "" &&
		this.scramUser != "" &&
		this.scramPasswordSecretName != ""

	this.mechOk = mechTopology == mechStorage

	this.foundScramMechanism = mechTopology
	// We won't actively delete old env values if not used
}

func (this *StreamsSecurityScramOcpCF) Compare() bool {
	// Condition #1
	return this.valid && (
		this.truststoreSecretName != this.foundTruststoreSecretName ||
			this.scramUser != this.foundScramUser ||
			this.scramPasswordSecretName != this.foundScramPasswordSecretName ||
			this.scramMechanism != this.foundScramMechanism ||
			!this.mechOk)
}

func (this *StreamsSecurityScramOcpCF) Respond() {
	this.AddEnv(this.truststoreSecretName, SCRAM_TRUSTSTORE_SECRET_VOLUME_NAME,
		this.scramUser, this.scramPasswordSecretName, this.scramMechanism)

	this.AddSecretVolumePatch(this.deploymentEntry, this.truststoreSecretName, SCRAM_TRUSTSTORE_SECRET_VOLUME_NAME)

	this.AddSecretMountPatch(this.deploymentEntry, SCRAM_TRUSTSTORE_SECRET_VOLUME_NAME, "etc/"+SCRAM_TRUSTSTORE_SECRET_VOLUME_NAME)
}

func (this *StreamsSecurityScramOcpCF) AddEnv(truststoreSecretName string, truststoreSecretVolumeName string,
	scramUser string, scramPasswordSecretName string, scramMechanism string) {

	this.ctx.GetEnvCache().Set(NewSimpleEnvCacheEntry(ENV_REGISTRY_PROPERTIES_PREFIX, "REGISTRY_"))

	this.ctx.GetEnvCache().Set(NewSimpleEnvCacheEntry(ENV_REGISTRY_STREAMS_SCRAM_USER, scramUser))
	this.ctx.GetEnvCache().Set(NewEnvCacheEntry(&core.EnvVar{
		Name: ENV_REGISTRY_STREAMS_SCRAM_PASSWORD,
		ValueFrom: &core.EnvVarSource{
			SecretKeyRef: &core.SecretKeySelector{
				LocalObjectReference: core.LocalObjectReference{
					Name: scramPasswordSecretName,
				},
				Key: "password",
			},
		},
	}))

	this.ctx.GetEnvCache().Set(NewSimpleEnvCacheEntry(ENV_REGISTRY_STREAMS_TOPOLOGY_SASL_MECHANISM, scramMechanism))

	jaasConfig := "org.apache.kafka.common.security.scram.ScramLoginModule required username=$(" + ENV_REGISTRY_STREAMS_SCRAM_USER +
		") password=$(" + ENV_REGISTRY_STREAMS_SCRAM_PASSWORD + ");"

	jaasconfigEntry := NewSimpleEnvCacheEntry(ENV_REGISTRY_STREAMS_TOPOLOGY_SASL_JAAS_CONFIG, jaasConfig)
	jaasconfigEntry.SetInterpolationDependency(ENV_REGISTRY_STREAMS_SCRAM_USER)
	jaasconfigEntry.SetInterpolationDependency(ENV_REGISTRY_STREAMS_SCRAM_PASSWORD)
	this.ctx.GetEnvCache().Set(jaasconfigEntry)

	this.ctx.GetEnvCache().Set(NewSimpleEnvCacheEntry(ENV_REGISTRY_STREAMS_TOPOLOGY_SECURITY_PROTOCOL, "SASL_SSL"))
	this.ctx.GetEnvCache().Set(NewSimpleEnvCacheEntry(ENV_REGISTRY_STREAMS_TOPOLOGY_SSL_TRUSTSTORE_TYPE, "PKCS12"))
	this.ctx.GetEnvCache().Set(NewSimpleEnvCacheEntry(ENV_REGISTRY_STREAMS_TOPOLOGY_SSL_TRUSTSTORE_LOCATION,
		"/etc/"+truststoreSecretVolumeName+"/ca.p12"))
	this.ctx.GetEnvCache().Set(NewEnvCacheEntry(&core.EnvVar{
		Name: ENV_REGISTRY_STREAMS_TOPOLOGY_SSL_TRUSTSTORE_PASSWORD,
		ValueFrom: &core.EnvVarSource{
			SecretKeyRef: &core.SecretKeySelector{
				LocalObjectReference: core.LocalObjectReference{
					Name: truststoreSecretName,
				},
				Key: "ca.password",
			},
		},
	}))

	this.ctx.GetEnvCache().Set(NewSimpleEnvCacheEntry(ENV_REGISTRY_STREAMS_STORAGE_PRODUCER_SASL_MECHANISM, scramMechanism))

	jaasconfigEntry = NewSimpleEnvCacheEntry(ENV_REGISTRY_STREAMS_STORAGE_PRODUCER_SASL_JAAS_CONFIG, jaasConfig)
	jaasconfigEntry.SetInterpolationDependency(ENV_REGISTRY_STREAMS_SCRAM_USER)
	jaasconfigEntry.SetInterpolationDependency(ENV_REGISTRY_STREAMS_SCRAM_PASSWORD)
	this.ctx.GetEnvCache().Set(jaasconfigEntry)

	this.ctx.GetEnvCache().Set(NewSimpleEnvCacheEntry(ENV_REGISTRY_STREAMS_STORAGE_PRODUCER_SECURITY_PROTOCOL, "SASL_SSL"))
	this.ctx.GetEnvCache().Set(NewSimpleEnvCacheEntry(ENV_REGISTRY_STREAMS_STORAGE_PRODUCER_SSL_TRUSTSTORE_TYPE, "PKCS12"))
	this.ctx.GetEnvCache().Set(NewSimpleEnvCacheEntry(ENV_REGISTRY_STREAMS_STORAGE_PRODUCER_SSL_TRUSTSTORE_LOCATION,
		"/etc/"+truststoreSecretVolumeName+"/ca.p12"))
	this.ctx.GetEnvCache().Set(NewEnvCacheEntry(&core.EnvVar{
		Name: ENV_REGISTRY_STREAMS_STORAGE_PRODUCER_SSL_TRUSTSTORE_PASSWORD,
		ValueFrom: &core.EnvVarSource{
			SecretKeyRef: &core.SecretKeySelector{
				LocalObjectReference: core.LocalObjectReference{
					Name: truststoreSecretName,
				},
				Key: "ca.password",
			},
		},
	}))

}

func (this *StreamsSecurityScramOcpCF) AddSecretVolumePatch(deploymentEntry ResourceCacheEntry, secretName string, volumeName string) {
	deploymentEntry.ApplyPatch(func(value interface{}) interface{} {
		deployment := value.(*ocp_apps.DeploymentConfig).DeepCopy()
		volume := core.Volume{
			Name: volumeName,
			VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}
		j := -1
		for i, v := range deployment.Spec.Template.Spec.Volumes {
			if v.Name == volumeName {
				j = i
				deployment.Spec.Template.Spec.Volumes[i] = volume
			}
		}
		if j == -1 {
			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, volume)
		}
		return deployment
	})
}

func (this *StreamsSecurityScramOcpCF) AddSecretMountPatch(deploymentEntry ResourceCacheEntry, volumeName string, mountPath string) {
	deploymentEntry.ApplyPatch(func(value interface{}) interface{} {
		deployment := value.(*ocp_apps.DeploymentConfig).DeepCopy()
		for ci, c := range deployment.Spec.Template.Spec.Containers {
			if c.Name == this.ctx.GetConfiguration().GetAppName() {
				mount := core.VolumeMount{
					Name:      volumeName,
					ReadOnly:  true,
					MountPath: mountPath,
				}
				j := -1
				for i, v := range deployment.Spec.Template.Spec.Containers[ci].VolumeMounts {
					if v.Name == volumeName {
						j = i
						deployment.Spec.Template.Spec.Containers[ci].VolumeMounts[i] = mount
					}
				}
				if j == -1 {
					deployment.Spec.Template.Spec.Containers[ci].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[ci].VolumeMounts, mount)
				}
			}
		}
		return deployment
	})
}
