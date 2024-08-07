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
	GetState() (WorkflowReport, error)
	Validate(args json.RawMessage) error
	AssignStateManager(state statemanager.StateManager)
}

var AvailableDeployments = []string{
	"git",
	"s3",
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
