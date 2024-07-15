package controller

import (
	"build-machine/internal"
	"build-machine/service"
	"build-machine/utils"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

type DeploymentsController interface {
	DeployFromS3Workflow(w http.ResponseWriter, r *http.Request)
	DeployFromGithubWorkflow(w http.ResponseWriter, r *http.Request)
	DeployEmptyProjectWorkflow(w http.ResponseWriter, r *http.Request)
	HealthCheck(w http.ResponseWriter, r *http.Request)
}

type deploymentsController struct {
	wfService service.WorkflowService
}

func NewDeploymentsController() DeploymentsController {
	return &deploymentsController{
		wfService: service.NewWorkflowService(),
	}
}

type ReqDeployFromS3WorkflowBody struct {
	Token       string            `json:"token"`
	Code        map[string]string `json:"code"`
	ProjectName string            `json:"projectName"`
	Region      string            `json:"region"`
	Stage       *string           `json:"stage,omitempty"`
	BasePath    *string           `json:"basePath,omitempty"`
}

func (d *deploymentsController) DeployFromS3Workflow(w http.ResponseWriter, r *http.Request) {
	var body ReqDeployFromS3WorkflowBody

	// Decode JSON body
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate JSON body
	if body.Region == "" {
		http.Error(w, "region is required", http.StatusBadRequest)
		return
	}

	if body.ProjectName == "" {
		http.Error(w, "projectName is required", http.StatusBadRequest)
		return
	}

	if body.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	if len(body.Code) == 0 {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}

	stage := "prod"
	if body.Stage != nil {
		stage = *body.Stage
	}

	tmpFolderPath := utils.CreateTempFolder()
	archivePath, err := writeCodeMapToDirAndZip(body.Code, tmpFolderPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpFolderPath)

	s3URLUpload, err := utils.UploadContentToS3(archivePath, body.ProjectName, body.Region, stage, body.Token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Parse upload s3 url to extract key
	s3ParsedURL, err := url.Parse(s3URLUpload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	uploadKey := strings.TrimLeft(s3ParsedURL.Path, "/")
	// Call service
	bucketBaseName := internal.GetConfig().BucketBaseName
	s3URLDownload, err := utils.DownloadFromS3PresignedURL(body.Region, fmt.Sprintf("%s-%s", bucketBaseName, body.Region), uploadKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Temporary: This will refresh the workflow service connection
	var wfService service.WorkflowService
	if internal.GetConfig().Env != "local" {
		wfService = service.NewWorkflowService()
	} else {
		wfService = d.wfService
	}
	workflowRes := wfService.S3Workflow(body.Token, s3URLDownload, body.ProjectName, body.Region, body.Stage, body.BasePath)
	w.Header().Add("Content-Type", "application/json")
	w.Write([]byte(workflowRes))
}

type ReqDeployFromGithubWorkflow struct {
	Token       string  `json:"token"`
	Repository  string  `json:"githubRepository"`
	ProjectName *string `json:"projectName"`
	Region      *string `json:"region"`
	BasePath    *string `json:"basePath"`
}

func (d *deploymentsController) DeployFromGithubWorkflow(w http.ResponseWriter, r *http.Request) {
	var body ReqDeployFromGithubWorkflow
	// Decode JSON body
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate JSON body
	if body.Repository == "" {
		http.Error(w, "repository is required", http.StatusBadRequest)
		return
	}

	if body.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	if body.ProjectName == nil {
		http.Error(w, "projectName is required", http.StatusBadRequest)
		return
	}

	if body.Region == nil {
		http.Error(w, "region is required", http.StatusBadRequest)
		return
	}

	// Temporary: This will refresh the workflow service connection
	var wfService service.WorkflowService
	if internal.GetConfig().Env != "local" {
		wfService = service.NewWorkflowService()
	} else {
		wfService = d.wfService
	}
	// Call service
	workflowRes := wfService.GithubWorkflow(
		body.Token,
		body.Repository,
		body.ProjectName,
		body.Region,
		body.BasePath,
	)
	w.Header().Add("Content-Type", "application/json")
	w.Write([]byte(workflowRes))
}

type ReqDeployEmptyflow struct {
	Token       string   `json:"token"`
	Repository  string   `json:"githubRepository"`
	ProjectName *string  `json:"projectName"`
	Region      *string  `json:"region"`
	BasePath    *string  `json:"basePath"`
	Stack       []string `json:"stack"`
}

func (d *deploymentsController) DeployEmptyProjectWorkflow(w http.ResponseWriter, r *http.Request) {
	var body ReqDeployEmptyflow
	// Decode JSON body
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate JSON body
	if body.Repository == "" {
		http.Error(w, "repository is required", http.StatusBadRequest)
		return
	}

	if body.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	if body.ProjectName == nil {
		http.Error(w, "projectName is required", http.StatusBadRequest)
		return
	}

	if body.Region == nil {
		http.Error(w, "region is required", http.StatusBadRequest)
		return
	}

	// Temporary: This will refresh the workflow service connection
	var wfService service.WorkflowService
	if internal.GetConfig().Env != "local" {
		wfService = service.NewWorkflowService()
	} else {
		wfService = d.wfService
	}

	// Parsed stack to csv
	var parsedStack *string = new(string)
	if len(body.Stack) > 0 {
		*parsedStack = strings.Join(body.Stack, ",")
	} else {
		parsedStack = nil

	}
	// Call service
	workflowRes := wfService.EmptyProjectWorkflow(
		body.Token,
		body.Repository,
		body.ProjectName,
		body.Region,
		body.BasePath,
		parsedStack,
	)
	w.Header().Add("Content-Type", "application/json")
	w.Write([]byte(workflowRes))
}

func (d *deploymentsController) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func writeCodeMapToDirAndZip(code map[string]string, tmpFolderPath string) (string, error) {
	// Write code to temp folder
	for fileName, fileContent := range code {
		filePath := path.Join(tmpFolderPath, fileName)
		log.Default().Println("Writing file", fileName, "to", tmpFolderPath)

		// Check if file is in a subfolder
		if strings.Contains(fileName, "/") {
			err := os.MkdirAll(path.Dir(filePath), 0755)
			if err != nil {
				return "", err
			}
		}

		err := os.WriteFile(filePath, []byte(fileContent), 0644)
		if err != nil {
			return "", err
		}
	}

	destinationPath := path.Join(tmpFolderPath, "projectCode.zip")

	if err := utils.ZipDirectory(tmpFolderPath, destinationPath); err != nil {
		return "", err
	}

	return destinationPath, nil
}
