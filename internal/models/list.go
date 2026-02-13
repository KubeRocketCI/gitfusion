package models

type ListOptions struct {
	Name *string
}

type PullRequestListOptions struct {
	State   string // "open", "closed", "merged", "all"
	Page    int
	PerPage int
}

type PipelineListOptions struct {
	Ref     *string // Filter by branch/tag ref
	Status  *string // Filter by normalized status
	Page    int
	PerPage int
}
