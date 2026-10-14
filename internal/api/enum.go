package api

import (
	"fmt"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
)

// enum is an oapi-codegen enum type; Valid reports membership in the oapi.yaml enum.
type enum interface {
	~string
	Valid() bool
}

// validateEnum returns an *InvalidParamError when value is set and not in its enum. Matching is case-sensitive.
func validateEnum[T enum](param string, value *T) error {
	if value == nil || (*value).Valid() {
		return nil
	}

	return &gferrors.InvalidParamError{
		Param: param,
		Err:   fmt.Errorf("unsupported value %q", *value),
	}
}
