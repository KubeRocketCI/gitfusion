package common

import (
	"fmt"
	"strings"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
)

// SplitProject splits a "owner/repo" (or "workspace/repo") string into its
// two components. It returns an error wrapping gferrors.ErrBadRequest when the
// input does not contain exactly two non-empty parts separated by "/".
func SplitProject(project string) (first, second string, err error) {
	parts := strings.SplitN(project, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid project format %q: expected \"owner/repo\": %w", project, gferrors.ErrBadRequest)
	}

	return parts[0], parts[1], nil
}
