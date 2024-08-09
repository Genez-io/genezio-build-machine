package workflows

import (
	statemanager "build-machine/state_manager"
	"encoding/json"
)

type WorkflowReport struct {
	JobName   string
	Status    string
	StartTime string
	EndTime   string
	Message   string
}

type Workflow interface {
	Submit() (string, error)
	Validate(args json.RawMessage) error
	AssignStateManager(state statemanager.StateManager)
	AssignEnvVarsFromStageID(envVars map[string]string)
}

var AvailableDeployments = []string{
	"git",
	"s3",
}

// Specific input definitions for each workflow type
type GitDeployment struct {
	Repository   string            `json:"githubRepository"`
	ProjectName  string            `json:"projectName"`
	Region       string            `json:"region"`
	Stage        string            `json:"stage"`
	BasePath     *string           `json:"basePath,omitempty"`
	Stack        []string          `json:"stack,omitempty"`
	IsNewProject bool              `json:"isNewProject"`
	EnvVars      map[string]string `json:"envVars,omitempty"`
}

type S3Deployment struct {
	S3DownloadURL string            `json:"s3DownloadURL,omitempty"`
	ProjectName   string            `json:"projectName"`
	Stage         string            `json:"stage"`
	Region        string            `json:"region"`
	BasePath      *string           `json:"basePath,omitempty"`
	Code          map[string]string `json:"code"`
	EnvVars       map[string]string `json:"envVars,omitempty"`
}

func GetWorkflowExecutor(workflow, token string) Workflow {
	switch workflow {
	case "git":
		return NewGitArgoWorkflow(token)
	case "s3":
		return NewS3ArgoDeployment(token)
	default:
		return nil
	}
}
