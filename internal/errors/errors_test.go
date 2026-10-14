package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvalidParamError_NamesParamAndUnwrapsCause(t *testing.T) {
	cause := errors.New("strconv.Atoi: parsing \"abc\": invalid syntax")

	var err error = &InvalidParamError{Param: "pipelineId", Err: cause}

	assert.Equal(t, `invalid parameter pipelineId: strconv.Atoi: parsing "abc": invalid syntax`, err.Error())
	assert.ErrorIs(t, err, cause)
}
