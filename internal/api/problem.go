package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v2"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
)

const (
	problemContentType = "application/problem+json"
	problemTypeBlank   = "about:blank"

	detailUnsupportedOperation = "The operation is not supported for this git server."
	detailNotFound             = "The requested resource was not found."
	detailUnexpected           = "The request could not be completed. Contact your administrator with the request ID."
)

// writeProblem writes err as an RFC 9457 problem response.
// The response carries a fixed detail and the request ID; err.Error() goes only to the request log line.
// err must not be nil.
func writeProblem(w http.ResponseWriter, r *http.Request, err error) {
	httplog.LogEntrySetField(r.Context(), "error", slog.StringValue(err.Error()))

	status, detail := classifyError(err)
	problem := models.Problem{
		Type:   problemTypeBlank,
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
	}

	if id := middleware.GetReqID(r.Context()); id != "" {
		w.Header().Set(middleware.RequestIDHeader, id)

		problem.RequestId = &id
	}

	w.Header().Set("Content-Type", problemContentType)
	w.WriteHeader(status)

	// Encode error ignored: headers already sent.
	_ = json.NewEncoder(w).Encode(problem)
}

// classifyError maps err to a response status and a fixed detail. The first matching rule wins.
func classifyError(err error) (status int, detail string) {
	var (
		required *RequiredParamError
		format   *InvalidParamFormatError
		invalid  *gferrors.InvalidParamError
	)

	switch {
	case errors.As(err, &required):
		return http.StatusBadRequest, fmt.Sprintf("Missing required parameter '%s'.", required.ParamName)
	case errors.As(err, &format):
		return http.StatusBadRequest, invalidParamDetail(format.ParamName)
	case errors.As(err, &invalid):
		return http.StatusBadRequest, invalidParamDetail(invalid.Param)
	case errors.Is(err, gferrors.ErrUnsupportedOperation):
		return http.StatusBadRequest, detailUnsupportedOperation
	case errors.Is(err, gferrors.ErrNotFound):
		return http.StatusNotFound, detailNotFound
	case errors.Is(err, gferrors.ErrUnauthorized):
		return http.StatusBadGateway, detailUnexpected
	default:
		return http.StatusInternalServerError, detailUnexpected
	}
}

func invalidParamDetail(param string) string {
	return fmt.Sprintf("Invalid value for parameter '%s'.", param)
}
