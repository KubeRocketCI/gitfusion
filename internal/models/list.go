package models

type ListOptions struct {
	Name *string
}

type PullRequestListOptions struct {
	State   string // "open", "closed", "merged", "all"
	Page    int
	PerPage int
}
