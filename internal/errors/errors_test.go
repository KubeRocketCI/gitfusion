package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvalidParamError_NamesParamAndUnwrapsCause(t *testing.T) {
	cause := errors.New("strconv.Atoi: parsing \"abc\": invalid syntax")

	var err error = &InvalidParamError{Param: "pipelineId", Err: cause}

	assert.Equal(t, `invalid parameter pipelineId: strconv.Atoi: parsing "abc": invalid syntax`, err.Error())
	assert.ErrorIs(t, err, cause)
}

func TestUnsupportedOperation_MatchesSentinelAndNamesProviderAndOperation(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", UnsupportedOperation("gerrit", "listRepositories"))

	assert.ErrorIs(t, err, ErrUnsupportedOperation)
	assert.NotErrorIs(t, err, ErrNotFound)
	assert.EqualError(t, err, "wrapped: provider gerrit does not support listRepositories: unsupported operation")
}
