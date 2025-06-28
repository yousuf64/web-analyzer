package api

import (
	"crypto/rand"
	"shared/models"
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
func getDefaultTasks(jobID string) []*models.Task {
	return []*models.Task{
		{JobID: jobID, Type: models.TaskTypeExtracting, Status: models.TaskStatusPending, SubTasks: make(map[string]models.SubTask)},
		{JobID: jobID, Type: models.TaskTypeIdentifyingVersion, Status: models.TaskStatusPending, SubTasks: make(map[string]models.SubTask)},
		{JobID: jobID, Type: models.TaskTypeAnalyzing, Status: models.TaskStatusPending, SubTasks: make(map[string]models.SubTask)},
		{JobID: jobID, Type: models.TaskTypeVerifyingLinks, Status: models.TaskStatusPending, SubTasks: make(map[string]models.SubTask)},
	}
}
