package gitlab

import (
	"testing"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/stretchr/testify/assert"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func TestConvertToPipelineVariables(t *testing.T) {
	tests := []struct {
		name      string
		variables []models.PipelineVariable
		wantNil   bool
		wantLen   int
	}{
		{
			name:      "empty variables returns nil",
			variables: []models.PipelineVariable{},
			wantNil:   true,
		},
		{
			name:      "nil variables returns nil",
			variables: nil,
			wantNil:   true,
		},
		{
			name: "single variable without type",
			variables: []models.PipelineVariable{
				{Key: "ENV", Value: "prod"},
			},
			wantNil: false,
			wantLen: 1,
		},
		{
			name: "multiple variables with types",
			variables: []models.PipelineVariable{
				{Key: "ENV", Value: "prod", VariableType: varTypePtr(models.EnvVar)},
				{Key: "CONFIG", Value: "/path/to/config", VariableType: varTypePtr(models.File)},
			},
			wantNil: false,
			wantLen: 2,
		},
		{
			name: "mixed types and nil types",
			variables: []models.PipelineVariable{
				{Key: "VAR1", Value: "value1", VariableType: varTypePtr(models.EnvVar)},
				{Key: "VAR2", Value: "value2", VariableType: nil},
			},
			wantNil: false,
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToPipelineVariables(tt.variables)

			if tt.wantNil {
				assert.Nil(t, got, "expected nil result")
				return
			}

			assert.NotNil(t, got, "expected non-nil result")
			assert.Equal(t, tt.wantLen, len(*got), "wrong number of variables")

			// Verify variable content
			for i, inputVar := range tt.variables {
				gotVar := (*got)[i]
				assert.NotNil(t, gotVar, "variable should not be nil")
				assert.Equal(t, inputVar.Key, *gotVar.Key, "key mismatch")
				assert.Equal(t, inputVar.Value, *gotVar.Value, "value mismatch")

				if inputVar.VariableType != nil {
					assert.NotNil(t, gotVar.VariableType, "variable type should be set")
					expectedType := gitlab.VariableTypeValue(*inputVar.VariableType)
					assert.Equal(t, expectedType, *gotVar.VariableType, "variable type mismatch")
				}
			}
		})
	}
}

// Helper function
func varTypePtr(v models.PipelineVariableVariableType) *models.PipelineVariableVariableType {
	return &v
}

func TestConvertToPipelineVariables_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		variables []models.PipelineVariable
		validate  func(t *testing.T, got *[]*gitlab.PipelineVariableOptions)
	}{
		{
			name: "empty string values",
			variables: []models.PipelineVariable{
				{Key: "", Value: ""},
			},
			validate: func(t *testing.T, got *[]*gitlab.PipelineVariableOptions) {
				assert.NotNil(t, got)
				assert.Equal(t, 1, len(*got))
				assert.Equal(t, "", *(*got)[0].Key)
				assert.Equal(t, "", *(*got)[0].Value)
			},
		},
		{
			name: "special characters in values",
			variables: []models.PipelineVariable{
				{Key: "SPECIAL", Value: "value with spaces and $pecial ch@rs!"},
			},
			validate: func(t *testing.T, got *[]*gitlab.PipelineVariableOptions) {
				assert.NotNil(t, got)
				assert.Equal(t, "value with spaces and $pecial ch@rs!", *(*got)[0].Value)
			},
		},
		{
			name: "large variable value",
			variables: []models.PipelineVariable{
				{Key: "LARGE", Value: string(make([]byte, 10000))},
			},
			validate: func(t *testing.T, got *[]*gitlab.PipelineVariableOptions) {
				assert.NotNil(t, got)
				assert.Equal(t, 10000, len(*(*got)[0].Value))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToPipelineVariables(tt.variables)
			tt.validate(t, got)
		})
	}
}
