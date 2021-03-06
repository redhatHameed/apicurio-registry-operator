package apicurioregistry

var _ ControlFunction = &ProfileCF{}

const ENV_QUARKUS_PROFILE = "QUARKUS_PROFILE"

type ProfileCF struct {
	ctx        *Context
	profileSet bool
}

// Is responsible for managing environment variables from the env cache
func NewProfileCF(ctx *Context) ControlFunction {
	return &ProfileCF{
		ctx:        ctx,
		profileSet: false,
	}
}

func (this *ProfileCF) Describe() string {
	return "ProfileCF"
}

func (this *ProfileCF) Sense() {
	// Observation #1
	// Was the profile env var set?
	_, profileSet := this.ctx.GetEnvCache().Get(ENV_QUARKUS_PROFILE)
	this.profileSet = profileSet

}

func (this *ProfileCF) Compare() bool {
	// Condition #1
	// Env var does not exist
	return !this.profileSet
}

func (this *ProfileCF) Respond() {
	// Response #1
	// Just set the value(s)!
	this.ctx.GetEnvCache().Set(NewSimpleEnvCacheEntry(ENV_QUARKUS_PROFILE, "prod"))

}
