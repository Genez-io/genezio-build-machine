package statemanager

import (
	"fmt"
	"time"
)

type LocalStateManager struct {
	// UserConcurrentBuilds keeps track of the number of concurrent builds a user has.
	// This allows us to enforce a maximum number of builds a user can have running at the same time.
	UserConcurrentBuilds map[string]int
	// BuildMap is a map of jobId to State, used for looking up or modifiying the state of a build.
	BuildMap map[string]*State
	// WebhookSecretRef is a map of webhook secret to jobId, used for looking up the jobId of a build when status is reported
	// via webhook by the build job.
	WebhookSecretRef map[string]string
}

// GetJobIdByWebhookSecretRef implements StateManager.
func (l *LocalStateManager) GetJobIdByWebhookSecretRef(webhookSecret string) (string, error) {
	if jobId, ok := l.WebhookSecretRef[webhookSecret]; ok {
		return jobId, nil
	}

	return "", fmt.Errorf("job_id not found")
}

// GetJobIdByWebhookSecretRef implements StateManager.
func (l *LocalStateManager) AttachJobIdToWebhookSecretRef(whSecret, job_id string) error {
	if job_id == "" {
		return fmt.Errorf("job_id is required")
	}
	if whSecret == "" {
		return fmt.Errorf("webhookSecret is required")
	}

	l.WebhookSecretRef[whSecret] = job_id
	return nil
}

// CreateState implements StateManager.
// CreateState creates a new state for a build, incrementing the user's concurrent builds.
// This function also allocates a channel that can be used for real time updates on the state of the build.
// Note that this function does not check if the user has exceeded the maximum number of concurrent builds.
func (l *LocalStateManager) CreateState(jobId, token string, engine string) error {
	if _, ok := l.UserConcurrentBuilds[token]; !ok {
		l.UserConcurrentBuilds[token] = 0
	}
	l.UserConcurrentBuilds[token] = l.UserConcurrentBuilds[token] + 1

	now := time.Now()
	l.BuildMap[jobId] = &State{
		BuildStatus: StatusPending,
		BuildEngine: engine,
		Timestamp:   now,
		UserToken:   token,
		Watcher:     make(chan State, 15),
		Transitions: []StateTransition{
			{
				From:           StatusProcessing,
				To:             StatusPending,
				TransitionTime: now,
				Reason:         "Build request successfully submitted, waiting for scheduling",
			},
		},
	}
	return nil
}

// GetConcurrentBuilds implements StateManager.
func (l *LocalStateManager) GetConcurrentBuilds(token string) int {
	if _, ok := l.UserConcurrentBuilds[token]; !ok {
		return 0
	}
	return l.UserConcurrentBuilds[token]
}

// GetState implements StateManager.
func (l *LocalStateManager) GetState(jobId string) (State, error) {
	if _, ok := l.BuildMap[jobId]; !ok {
		return State{}, fmt.Errorf("job doesn't exist")
	}
	return *l.BuildMap[jobId], nil
}

// UpdateState implements StateManager.
func (l *LocalStateManager) UpdateState(jobId, reason string, state BuildStatus) error {
	if _, ok := l.BuildMap[jobId]; !ok {
		return fmt.Errorf("job doesn't exist")
	}
	now := time.Now()
	oldState := *l.BuildMap[jobId]
	newTransition := StateTransition{
		From:           oldState.BuildStatus,
		To:             state,
		TransitionTime: now,
		Reason:         reason,
	}
	l.BuildMap[jobId].BuildStatus = state
	l.BuildMap[jobId].Transitions = append(l.BuildMap[jobId].Transitions, newTransition)
	l.BuildMap[jobId].Timestamp = now

	// Notify any watchers
	l.BuildMap[jobId].Watcher <- *l.BuildMap[jobId]

	if state == StatusSuccess || state == StatusFailed {
		l.UserConcurrentBuilds[oldState.UserToken]--
		close(l.BuildMap[jobId].Watcher)
	}

	return nil
}

func NewLocalStateManager() StateManager {
	userConcurrentBuilds := make(map[string]int)
	buildMap := make(map[string]*State)
	whSecretRef := make(map[string]string)
	return &LocalStateManager{
		UserConcurrentBuilds: userConcurrentBuilds,
		BuildMap:             buildMap,
		WebhookSecretRef:     whSecretRef,
	}
}
