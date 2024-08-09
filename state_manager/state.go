package statemanager

import (
	"time"
)

type BuildStatus string

const (
	StatusProcessing        BuildStatus = "PROCESSING"
	StatusPending           BuildStatus = "PENDING"
	StatusScheduled         BuildStatus = "SCHEDULED"
	StatusAuth              BuildStatus = "AUTHENTICATING"
	StatusPullingCode       BuildStatus = "PULLING_CODE"
	StatusInstallingDeps    BuildStatus = "INSTALLING_DEPS"
	StatusBuilding          BuildStatus = "BUILDING"
	StatusDeployingBackend  BuildStatus = "DEPLOYING_BACKEND"
	StatusDeployingFrontend BuildStatus = "DEPLOYING_FRONTEND"
	StatusSuccess           BuildStatus = "SUCCEEDED"
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
	Watcher     chan State `json:"-"`
}

type SelfReportedState struct {
	Status  string    `json:"status"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

type StateManager interface {
	CreateState(jobId, token, engine string) error
	GetJobIdByWebhookSecretRef(webhookSecret string) (string, error)
	AttachJobIdToWebhookSecretRef(whSecret, job_id string) error
	GetState(jobId string) (State, error)
	UpdateState(jobId, reason string, state BuildStatus) error
	GetConcurrentBuilds(token string) int
}
