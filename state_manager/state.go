package statemanager

import "time"

type BuildStatus string

const (
	StatusPending           BuildStatus = "PENDING"
	StatusAuth              BuildStatus = "AUTHENTICATING"
	StatusPullingCode       BuildStatus = "PULLING_CODE"
	StatusInstallingDeps    BuildStatus = "INSTALLING_DEPS"
	StatusBuilding          BuildStatus = "BUILDING"
	StatusDeployingBackend  BuildStatus = "DEPLOYING_BACKEND"
	StatusDeployingFrontend BuildStatus = "DEPLOYING_FRONTEND"
	StatusSuccess           BuildStatus = "SUCCESS"
	StatusFailed            BuildStatus = "FAILED"
)

const (
	EngineArgo = "argo"
	EngineAPI  = "api"
)

type StateTransition struct {
	From           BuildStatus
	To             BuildStatus
	TransitionTime time.Time
	Reason         string
}

type State struct {
	BuildEngine string
	BuildStatus BuildStatus
	Timestamp   time.Time
	UserToken   string
	Transitions []StateTransition
}

type StateManager interface {
	CreateState(jobId, token string, engine string) error
	GetState(jobId string) (State, error)
	UpdateState(jobId, reason string, state BuildStatus) error
	GetConcurrentBuilds(token string) int
}
