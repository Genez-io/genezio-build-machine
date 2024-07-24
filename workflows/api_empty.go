package workflows

import (
	"build-machine/internal"
	sm "build-machine/state_manager"
	"build-machine/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/go-git/go-git/v5"
	"github.com/google/uuid"
)

type EmptyDeploymentAPI struct {
	EmptyDeployment
	Token        string
	StateManager sm.StateManager
}

// AssignStateManager implements Workflow.
func (e *EmptyDeploymentAPI) AssignStateManager(state sm.StateManager) {
	e.StateManager = state
}

// GetState implements Workflow.
func (e *EmptyDeploymentAPI) GetState() (WorkflowReport, error) {
	panic("unimplemented")
}

// Submit implements Workflow.
func (e *EmptyDeploymentAPI) Submit() (string, error) {
	jobId := uuid.NewString()
	err := e.StateManager.CreateState(jobId, e.Token, "api")
	if err != nil {
		return "", err
	}

	e.StateManager.UpdateState(jobId, "Cloning repository", sm.StatusPullingCode)
	// Clone repository
	clonePath := utils.CreateTempFolder()
	_, err = git.PlainClone(clonePath, false, &git.CloneOptions{
		URL: e.Repository,
	})
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(clonePath)

	e.StateManager.UpdateState(jobId, "Auth and creating empty project entry", sm.StatusAuth)
	// Do a http call to the backend to create a new empty project entry
	genezioCreateProjectBody := GenezioCreateProjectBody{
		ProjectName:   e.ProjectName,
		Region:        e.Region,
		CloudProvider: "genezio-cloud",
		Stage:         "prod",
		Stack:         e.Stack,
	}

	genezioCreateProjectBodyBytes, err := json.Marshal(genezioCreateProjectBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/core/deployment", internal.GetConfig().BackendURL), bytes.NewBuffer(genezioCreateProjectBodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", e.Token))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept-Version", "genezio-cli/2.0.3")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to create empty project: %s", resp.Status)
	}

	e.StateManager.UpdateState(jobId, "Started code packaging", sm.StatusBuilding)
	// Zip git directory and upload to s3
	zipDestinationPath := utils.CreateTempFolder()
	err = utils.ZipDirectory(clonePath, path.Join(zipDestinationPath, "projectCode.zip"))
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(zipDestinationPath)
	// Upload to s3
	_, err = utils.UploadContentToS3(path.Join(zipDestinationPath, "projectCode.zip"), e.ProjectName, e.Region, "prod", e.Token)
	if err != nil {
		e.StateManager.UpdateState(jobId, "Project code upload fail ", sm.StatusFailed)
		return "", err
	}
	e.StateManager.UpdateState(jobId, "Project code upload success", sm.StatusSuccess)
	return jobId, nil
}

// Validate implements Workflow.
func (d *EmptyDeploymentAPI) Validate(args json.RawMessage) error {
	err := json.Unmarshal(args, &d)
	if err != nil {
		return err
	}

	if d.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	if d.ProjectName == "" {
		return fmt.Errorf("projectName is required")
	}

	if d.Region == "" {
		return fmt.Errorf("region is required")
	}

	return nil
}

type GenezioCreateProjectBody struct {
	ProjectName   string   `json:"projectName"`
	Region        string   `json:"region"`
	CloudProvider string   `json:"cloudProvider"`
	Stage         string   `json:"stage"`
	Stack         []string `json:"stack"`
}

func NewEmptyProjectAPIWorkflow(token string) Workflow {
	return &EmptyDeploymentAPI{
		Token: token,
	}
}
