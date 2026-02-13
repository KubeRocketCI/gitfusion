package api

// clampPagination applies default values and bounds to pagination parameters.
// Default page is 1, default perPage is 20, max perPage is 100.
func clampPagination(page, perPage *int) (int, int) {
	p := 1
	if page != nil {
		p = *page
	}

	pp := 20
	if perPage != nil {
		pp = *perPage
		if pp > 100 {
			pp = 100
		}
	}

	if p < 1 {
		p = 1
	}

	if pp < 1 {
		pp = 20
	}

	return p, pp
}
