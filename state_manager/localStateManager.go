package statemanager

import (
	"fmt"
	"time"
)

type LocalStateManager struct {
	// a map that can be used for quick
	UserConcurrentBuilds map[string]int
	BuildMap             map[string]*State
}

// CreateState implements StateManager.
func (l *LocalStateManager) CreateState(jobId, token string, engine string) error {
	l.UserConcurrentBuilds[token] = 0
	l.BuildMap[jobId] = &State{
		BuildStatus: StatusPending,
		BuildEngine: engine,
		Timestamp:   time.Now(),
		UserToken:   token,
		Transitions: make([]StateTransition, 0),
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

	if state == StatusSuccess || state == StatusFailed {
		l.UserConcurrentBuilds[oldState.UserToken]--
	}
	return nil
}

func NewLocalStateManager() StateManager {
	userConcurrentBuilds := make(map[string]int)
	buildMap := make(map[string]*State)
	return &LocalStateManager{
		UserConcurrentBuilds: userConcurrentBuilds,
		BuildMap:             buildMap,
	}
}
