package api

import (
	"crypto/rand"
	"shared/types"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// entropyPool is a pool of ulid.MonotonicEntropy
var entropyPool = sync.Pool{
	New: func() any {
		return ulid.Monotonic(rand.Reader, 0)
	},
}

// generateID generates a new ULID
func generateID() string {
	e := entropyPool.Get().(*ulid.MonotonicEntropy)
	defer entropyPool.Put(e)
	ts := ulid.Timestamp(time.Now())
	return ulid.MustNew(ts, e).String()
}

// getDefaultTasks returns the default tasks for a job
func getDefaultTasks(jobID string) []*types.Task {
	return []*types.Task{
		{JobID: jobID, Type: types.TaskTypeExtracting, Status: types.TaskStatusPending},
		{JobID: jobID, Type: types.TaskTypeIdentifyingVersion, Status: types.TaskStatusPending},
		{JobID: jobID, Type: types.TaskTypeAnalyzing, Status: types.TaskStatusPending},
		{JobID: jobID, Type: types.TaskTypeVerifyingLinks, Status: types.TaskStatusPending},
	}
}
