package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/KubeRocketCI/gitfusion/pkg/pointer"
)

func TestClampPagination(t *testing.T) {
	tests := []struct {
		name        string
		page        *int
		perPage     *int
		wantPage    int
		wantPerPage int
	}{
		{
			name:        "nil page and perPage use defaults",
			page:        nil,
			perPage:     nil,
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name:        "custom page and perPage are used",
			page:        pointer.To(3),
			perPage:     pointer.To(50),
			wantPage:    3,
			wantPerPage: 50,
		},
		{
			name:        "perPage is capped at 100",
			page:        nil,
			perPage:     pointer.To(200),
			wantPage:    1,
			wantPerPage: 100,
		},
		{
			name:        "perPage at exactly 100 is not capped",
			page:        nil,
			perPage:     pointer.To(100),
			wantPage:    1,
			wantPerPage: 100,
		},
		{
			name:        "zero page is clamped to 1",
			page:        pointer.To(0),
			perPage:     nil,
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name:        "negative page is clamped to 1",
			page:        pointer.To(-5),
			perPage:     nil,
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name:        "zero perPage is clamped to 20",
			page:        nil,
			perPage:     pointer.To(0),
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name:        "negative perPage is clamped to 20",
			page:        nil,
			perPage:     pointer.To(-10),
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name:        "both zero page and zero perPage are clamped to defaults",
			page:        pointer.To(0),
			perPage:     pointer.To(0),
			wantPage:    1,
			wantPerPage: 20,
		},
		{
			name:        "perPage of 101 is capped to 100",
			page:        pointer.To(1),
			perPage:     pointer.To(101),
			wantPage:    1,
			wantPerPage: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPage, gotPerPage := clampPagination(tt.page, tt.perPage)

			assert.Equal(t, tt.wantPage, gotPage)
			assert.Equal(t, tt.wantPerPage, gotPerPage)
		})
	}
}
