package controller

import (
	"build-machine/internal"
	"build-machine/service"
	statemanager "build-machine/state_manager"
	"build-machine/workflows"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type DeploymentsController interface {
	Deploy(w http.ResponseWriter, r *http.Request)
	GetState(w http.ResponseWriter, r *http.Request)
	HealthCheck(w http.ResponseWriter, r *http.Request)
}

type deploymentsController struct {
	argoService  *service.ArgoService
	stateManager statemanager.StateManager
}

func NewDeploymentsController() DeploymentsController {
	stateManager := statemanager.NewLocalStateManager()
	return &deploymentsController{
		argoService:  service.NewArgoService(),
		stateManager: stateManager,
	}
}

type ResGetState struct {
	BuildEngine string
	BuildStatus statemanager.BuildStatus
	Timestamp   time.Time
	Transitions []statemanager.StateTransition
}

// GetState implements DeploymentsController.
func (d *deploymentsController) GetState(w http.ResponseWriter, r *http.Request) {
	job_id := r.PathValue("job_id")
	if job_id == "" {
		http.Error(w, "job_id is required", http.StatusBadRequest)
		return
	}
	// extract bearer token from Authorization header
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "Authorization header is required", http.StatusBadRequest)
		return
	}
	// Drop the "Bearer " prefix
	token = token[7:]

	job_state, err := d.stateManager.GetState(job_id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if job_state.UserToken != token {
		http.Error(w, "job_id not found", http.StatusNotFound)
		return
	}
	res := ResGetState{
		BuildEngine: job_state.BuildEngine,
		BuildStatus: job_state.BuildStatus,
		Timestamp:   job_state.Timestamp,
		Transitions: job_state.Transitions,
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

type ReqDeploy struct {
	Token string          `json:"token"`
	Type  string          `json:"type"`
	Stage string          `json:"stage"`
	Args  json.RawMessage `json:"args"`
}

type ResDeploy struct {
	JobID  string `json:"jobID"`
	Status string `json:"status"`
}

func (d *deploymentsController) Deploy(w http.ResponseWriter, r *http.Request) {
	var body ReqDeploy
	// Decode JSON body
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if body.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}
	if body.Type == "" {
		http.Error(w, fmt.Sprintf("type is required, one of [%v]", workflows.AvailableDeployments), http.StatusBadRequest)
		return
	}
	if body.Args == nil {
		http.Error(w, "args is required", http.StatusBadRequest)
		return
	}
	workflowExecutor := workflows.GetWorkflowExecutor(body.Type, body.Token)
	if workflowExecutor == nil {
		http.Error(w, fmt.Sprintf("type is required, one of [%v]", workflows.AvailableDeployments), http.StatusBadRequest)
		return
	}

	workflowExecutor.AssignStateManager(d.stateManager)

	maxConcurrentBuilds, err := strconv.ParseInt(internal.GetConfig().MaxConcurrentBuilds, 10, 64)
	if err != nil {
		http.Error(w, "failed to parse MAX_CONCURRENT_BUILDS", http.StatusInternalServerError)
		return
	}
	userCurrentBuildCount := d.stateManager.GetConcurrentBuilds(body.Token)
	if userCurrentBuildCount >= int(maxConcurrentBuilds) {
		http.Error(w, fmt.Sprintf("user has reached the maximum concurrent builds of %d", maxConcurrentBuilds), http.StatusBadRequest)
		return
	}

	if err := workflowExecutor.Validate(body.Args); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	job_id, err := workflowExecutor.Submit()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	job_state, err := d.stateManager.GetState(job_id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	res := ResDeploy{
		JobID:  job_id,
		Status: string(job_state.BuildStatus),
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

func (d *deploymentsController) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
