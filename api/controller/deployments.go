package controller

import (
	"build-machine/internal"
	"build-machine/service"
	statemanager "build-machine/state_manager"
	"build-machine/workflows"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

type DeploymentsController interface {
	Deploy(w http.ResponseWriter, r *http.Request)
	GetState(w http.ResponseWriter, r *http.Request)
	StreamState(w http.ResponseWriter, r *http.Request)
	HealthCheck(w http.ResponseWriter, r *http.Request)
	ReceiveStatusUpdate(w http.ResponseWriter, r *http.Request)
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

// ReceiveStatusUpdate implements DeploymentsController.
func (d *deploymentsController) ReceiveStatusUpdate(w http.ResponseWriter, r *http.Request) {
	var body statemanager.SelfReportedState
	// Decode JSON body
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// extract bearer whSecret from Authorization header
	whSecret := r.Header.Get("Authorization")
	if whSecret == "" {
		log.Println("Authorization header is required")
		http.Error(w, "Authorization header is required", http.StatusBadRequest)
		return
	}

	// Drop the "Bearer " prefix
	whSecret = whSecret[7:]

	job_id, err := d.stateManager.GetJobIdByWebhookSecretRef(whSecret)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = d.stateManager.GetState(job_id)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Update state
	err = d.stateManager.UpdateState(job_id, body.Message, statemanager.BuildStatus(body.Status))
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("Job [" + job_id + "] successfully reported status update [" + body.Status + "].")
	w.WriteHeader(http.StatusOK)
}

type ResGetState struct {
	BuildEngine string
	BuildStatus statemanager.BuildStatus
	Timestamp   time.Time
	Transitions []statemanager.StateTransition
}

// StreamState implements DeploymentsController.
func (d *deploymentsController) StreamState(w http.ResponseWriter, r *http.Request) {
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

	// Extract state
	job_state, err := d.stateManager.GetState(job_id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if job_state.UserToken != token {
		http.Error(w, "job_id not found for this user token", http.StatusNotFound)
		return
	}

	if job_state.Watcher == nil {
		http.Error(w, "job status watcher not available", http.StatusInternalServerError)
		return
	}

	// stream an incrementing counter
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// If job has already been completed stream the complete state once
	if job_state.BuildStatus == statemanager.StatusSuccess || job_state.BuildStatus == statemanager.StatusFailed {
		stateSerialized, err := json.Marshal(job_state)
		if err != nil {
			log.Println(err)
		}
		w.Write([]byte(fmt.Sprintf("data: %s\n\n", stateSerialized)))
		w.(http.Flusher).Flush()
	}

	// Whenever a state update occurs send it by streaming
	for s := range job_state.Watcher {
		stateSerialized, err := json.Marshal(s)
		if err != nil {
			log.Println(err)
		}
		w.Write([]byte(fmt.Sprintf("data: %s\n\n", stateSerialized)))
		w.(http.Flusher).Flush()
	}

	fmt.Printf("Finished HTTP stream for job [%s] at [%v]\n\n", job_id, time.Now())
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
		http.Error(w, "job_id not found for this user token", http.StatusNotFound)
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
