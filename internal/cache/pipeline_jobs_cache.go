package cache

import (
	"time"

	"github.com/viccon/sturdyc"

	"github.com/KubeRocketCI/gitfusion/internal/models"
)

// Pipeline-jobs cache: a single short-TTL tier, not status-aware. A finished GitLab pipeline can be
// retried (resurrected with new jobs), so an "all jobs terminal" snapshot must not be cached long.
// Traces differ: they key on an immutable job ID, so their done tier is safe.
const (
	jobsTTL  = 30 * time.Second
	jobsSize = 500
)

func NewPipelineJobsCache() *sturdyc.Client[[]models.PipelineJob] {
	numShards := 8
	evictionPercentage := 10

	return sturdyc.New[[]models.PipelineJob](jobsSize, numShards, jobsTTL, evictionPercentage)
}
