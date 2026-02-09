package bitbucket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_convertBitbucketPRState(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  models.PullRequestState
	}{
		{
			name:  "OPEN maps to open",
			state: "OPEN",
			want:  models.PullRequestStateOpen,
		},
		{
			name:  "MERGED maps to merged",
			state: "MERGED",
			want:  models.PullRequestStateMerged,
		},
		{
			name:  "DECLINED maps to closed",
			state: "DECLINED",
			want:  models.PullRequestStateClosed,
		},
		{
			name:  "SUPERSEDED maps to closed",
			state: "SUPERSEDED",
			want:  models.PullRequestStateClosed,
		},
		{
			name:  "unknown defaults to open",
			state: "unknown",
			want:  models.PullRequestStateOpen,
		},
		{
			name:  "empty defaults to open",
			state: "",
			want:  models.PullRequestStateOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertBitbucketPRState(tt.state)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_bitbucketPRResponse_JSONDeserialization(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    bitbucketPRResponse
		wantErr bool
	}{
		{
			name: "full response with single PR",
			input: `{
				"size": 1,
				"page": 1,
				"pagelen": 20,
				"values": [{
					"id": 42,
					"title": "Add feature",
					"state": "OPEN",
					"author": {
						"display_name": "John Doe",
						"uuid": "{user-uuid-123}",
						"links": {
							"avatar": {
								"href": "https://bitbucket.org/avatar.png"
							}
						}
					},
					"source": {
						"branch": {"name": "feature/new"}
					},
					"destination": {
						"branch": {"name": "main"}
					},
					"links": {
						"html": {
							"href": "https://bitbucket.org/owner/repo/pull-requests/42"
						}
					},
					"created_on": "2026-01-15T10:30:00.123456+00:00",
					"updated_on": "2026-01-16T14:00:00.654321+00:00"
				}]
			}`,
			want: bitbucketPRResponse{
				Size:    1,
				Page:    1,
				Pagelen: 20,
				Values: []bitbucketPR{
					{
						ID:    42,
						Title: "Add feature",
						State: "OPEN",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty values list",
			input: `{
				"size": 0,
				"page": 1,
				"pagelen": 20,
				"values": []
			}`,
			want: bitbucketPRResponse{
				Size:    0,
				Page:    1,
				Pagelen: 20,
				Values:  []bitbucketPR{},
			},
			wantErr: false,
		},
		{
			name:    "invalid JSON returns error",
			input:   `{invalid`,
			want:    bitbucketPRResponse{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bitbucketPRResponse
			err := json.Unmarshal([]byte(tt.input), &got)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want.Size, got.Size)
			assert.Equal(t, tt.want.Page, got.Page)
			assert.Equal(t, tt.want.Pagelen, got.Pagelen)
			assert.Equal(t, len(tt.want.Values), len(got.Values))

			if len(tt.want.Values) > 0 {
				assert.Equal(t, tt.want.Values[0].ID, got.Values[0].ID)
				assert.Equal(t, tt.want.Values[0].Title, got.Values[0].Title)
				assert.Equal(t, tt.want.Values[0].State, got.Values[0].State)
			}
		})
	}
}

func Test_bitbucketPR_FieldMapping(t *testing.T) {
	input := `{
		"id": 99,
		"title": "Fix bug in parser",
		"state": "MERGED",
		"author": {
			"display_name": "Jane Smith",
			"uuid": "{jane-uuid}",
			"links": {
				"avatar": {
					"href": "https://bitbucket.org/jane-avatar.png"
				}
			}
		},
		"source": {
			"branch": {"name": "bugfix/parser"}
		},
		"destination": {
			"branch": {"name": "develop"}
		},
		"links": {
			"html": {
				"href": "https://bitbucket.org/owner/repo/pull-requests/99"
			}
		},
		"created_on": "2026-02-01T08:15:30.000000+00:00",
		"updated_on": "2026-02-02T12:45:00.000000+00:00"
	}`

	var pr bitbucketPR

	err := json.Unmarshal([]byte(input), &pr)
	require.NoError(t, err)

	assert.Equal(t, 99, pr.ID)
	assert.Equal(t, "Fix bug in parser", pr.Title)
	assert.Equal(t, "MERGED", pr.State)
	assert.Equal(t, "Jane Smith", pr.Author.DisplayName)
	assert.Equal(t, "{jane-uuid}", pr.Author.UUID)
	assert.Equal(t, "https://bitbucket.org/jane-avatar.png", pr.Author.Links.Avatar.Href)
	assert.Equal(t, "bugfix/parser", pr.Source.Branch.Name)
	assert.Equal(t, "develop", pr.Destination.Branch.Name)
	assert.Equal(t, "https://bitbucket.org/owner/repo/pull-requests/99", pr.Links.HTML.Href)
	assert.Equal(t, "2026-02-01T08:15:30.000000+00:00", pr.CreatedOn)
	assert.Equal(t, "2026-02-02T12:45:00.000000+00:00", pr.UpdatedOn)
}

func testBitbucketToken() string {
	return base64.StdEncoding.EncodeToString([]byte("user:pass"))
}

// newTestBitbucketPR creates a bitbucketPR with the given fields populated.
// For fields like Author and Links that are rarely needed, callers can set them
// directly on the returned value.
func newTestBitbucketPR(id int, title, state, sourceBranch, createdOn, updatedOn string) bitbucketPR {
	return bitbucketPR{
		ID:    id,
		Title: title,
		State: state,
		Source: struct {
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
		}{Branch: struct {
			Name string `json:"name"`
		}{Name: sourceBranch}},
		Destination: struct {
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
		}{Branch: struct {
			Name string `json:"name"`
		}{Name: "main"}},
		CreatedOn: createdOn,
		UpdatedOn: updatedOn,
	}
}

// redirectTransport redirects all HTTP requests to the test server.
type redirectTransport struct {
	target  string
	wrapped http.RoundTripper
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	targetURL, _ := url.Parse(t.target)
	req.URL.Scheme = targetURL.Scheme
	req.URL.Host = targetURL.Host

	return t.wrapped.RoundTrip(req)
}

func TestBitbucketService_ListPullRequests(t *testing.T) {
	tests := []struct {
		name           string
		state          string
		responseBody   bitbucketPRResponse
		statusCode     int
		wantErr        bool
		wantErrContain string
		wantCount      int
		wantState      models.PullRequestState
		validateQuery  func(t *testing.T, r *http.Request)
	}{
		{
			name:  "open state maps to OPEN query param",
			state: "open",
			responseBody: bitbucketPRResponse{
				Size:    1,
				Page:    1,
				Pagelen: 20,
				Values: []bitbucketPR{
					newTestBitbucketPR(1, "Open PR", "OPEN", "feature",
						"2026-01-15T10:30:00.000000+00:00", "2026-01-16T14:00:00.000000+00:00"),
				},
			},
			statusCode: http.StatusOK,
			wantCount:  1,
			wantState:  models.PullRequestStateOpen,
			validateQuery: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "OPEN", r.URL.Query().Get("state"))
			},
		},
		{
			name:  "closed state maps to DECLINED query param",
			state: "closed",
			responseBody: bitbucketPRResponse{
				Size:    1,
				Page:    1,
				Pagelen: 20,
				Values: []bitbucketPR{
					newTestBitbucketPR(2, "Declined PR", "DECLINED", "old-feature",
						"2026-01-10T10:00:00.000000+00:00", "2026-01-11T10:00:00.000000+00:00"),
				},
			},
			statusCode: http.StatusOK,
			wantCount:  1,
			wantState:  models.PullRequestStateClosed,
			validateQuery: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "DECLINED", r.URL.Query().Get("state"))
			},
		},
		{
			name:  "merged state maps to MERGED query param",
			state: "merged",
			responseBody: bitbucketPRResponse{
				Size:    1,
				Page:    1,
				Pagelen: 20,
				Values: []bitbucketPR{
					newTestBitbucketPR(3, "Merged PR", "MERGED", "completed-feature",
						"2026-01-05T10:00:00.000000+00:00", "2026-01-06T10:00:00.000000+00:00"),
				},
			},
			statusCode: http.StatusOK,
			wantCount:  1,
			wantState:  models.PullRequestStateMerged,
			validateQuery: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "MERGED", r.URL.Query().Get("state"))
			},
		},
		{
			name:  "all state sends multiple state params",
			state: "all",
			responseBody: bitbucketPRResponse{
				Size:    0,
				Page:    1,
				Pagelen: 20,
				Values:  []bitbucketPR{},
			},
			statusCode: http.StatusOK,
			wantCount:  0,
			validateQuery: func(t *testing.T, r *http.Request) {
				states := r.URL.Query()["state"]
				assert.Contains(t, states, "OPEN")
				assert.Contains(t, states, "MERGED")
				assert.Contains(t, states, "DECLINED")
				assert.Contains(t, states, "SUPERSEDED")
				assert.Equal(t, 4, len(states))
			},
		},
		{
			name:  "default state sends OPEN",
			state: "invalid_state",
			responseBody: bitbucketPRResponse{
				Size:    0,
				Page:    1,
				Pagelen: 20,
				Values:  []bitbucketPR{},
			},
			statusCode: http.StatusOK,
			wantCount:  0,
			validateQuery: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "OPEN", r.URL.Query().Get("state"))
			},
		},
		{
			name:           "non-200 response returns error",
			state:          "open",
			responseBody:   bitbucketPRResponse{},
			statusCode:     http.StatusUnauthorized,
			wantErr:        true,
			wantErrContain: "status 401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedReq *http.Request

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedReq = r

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)

				body, _ := json.Marshal(tt.responseBody)
				_, _ = w.Write(body)
			}))
			defer server.Close()

			svc := &BitbucketService{
				httpClient: resty.New().SetTransport(&redirectTransport{
					target:  server.URL,
					wrapped: http.DefaultTransport,
				}),
			}
			token := testBitbucketToken()

			result, err := svc.ListPullRequests(
				context.Background(),
				"owner",
				"repo",
				krci.GitServerSettings{Token: token},
				models.PullRequestListOptions{
					State:   tt.state,
					Page:    1,
					PerPage: 20,
				},
			)

			if tt.wantErr {
				assert.Error(t, err)

				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantCount, len(result.Data))

			if tt.wantCount > 0 {
				assert.Equal(t, tt.wantState, result.Data[0].State)
			}

			if tt.validateQuery != nil && capturedReq != nil {
				tt.validateQuery(t, capturedReq)
			}
		})
	}
}

func TestBitbucketService_ListPullRequests_FieldMapping(t *testing.T) {
	bbPR := newTestBitbucketPR(42, "Add new feature", "OPEN", "feature/awesome",
		"2026-01-15T10:30:00.123456+00:00", "2026-01-16T14:00:00.654321+00:00")
	bbPR.Author.DisplayName = "John Doe"
	bbPR.Author.UUID = "{user-uuid}"
	bbPR.Author.Links.Avatar.Href = "https://avatar.example.com/john.png"
	bbPR.Links.HTML.Href = "https://bitbucket.org/owner/repo/pull-requests/42"

	prResponse := bitbucketPRResponse{
		Size:    1,
		Page:    1,
		Pagelen: 20,
		Values:  []bitbucketPR{bbPR},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		body, _ := json.Marshal(prResponse)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}
	token := testBitbucketToken()

	page := 1
	perPage := 20

	result, err := svc.ListPullRequests(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: token},
		models.PullRequestListOptions{State: "open", Page: page, PerPage: perPage},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)

	pr := result.Data[0]
	assert.Equal(t, "42", pr.Id)
	assert.Equal(t, 42, pr.Number)
	assert.Equal(t, "Add new feature", pr.Title)
	assert.Equal(t, models.PullRequestStateOpen, pr.State)
	assert.Equal(t, "feature/awesome", pr.SourceBranch)
	assert.Equal(t, "main", pr.TargetBranch)
	assert.Equal(t, "https://bitbucket.org/owner/repo/pull-requests/42", pr.Url)

	// Author mapping
	require.NotNil(t, pr.Author)
	assert.Equal(t, "{user-uuid}", pr.Author.Id)
	assert.Equal(t, "John Doe", pr.Author.Name)
	require.NotNil(t, pr.Author.AvatarUrl)
	assert.Equal(t, "https://avatar.example.com/john.png", *pr.Author.AvatarUrl)

	// Timestamps parsed with RFC3339Nano (microsecond precision)
	assert.Equal(t, 2026, pr.CreatedAt.Year())
	assert.Equal(t, 1, int(pr.CreatedAt.Month()))
	assert.Equal(t, 15, pr.CreatedAt.Day())
	assert.Equal(t, 10, pr.CreatedAt.Hour())
	assert.Equal(t, 30, pr.CreatedAt.Minute())
	assert.Equal(t, 123456000, pr.CreatedAt.Nanosecond())

	// Pagination
	assert.Equal(t, 1, result.Pagination.Total)
	require.NotNil(t, result.Pagination.Page)
	assert.Equal(t, page, *result.Pagination.Page)
	require.NotNil(t, result.Pagination.PerPage)
	assert.Equal(t, perPage, *result.Pagination.PerPage)
}

func TestBitbucketService_ListPullRequests_EmptyAuthorAvatar(t *testing.T) {
	bbPR := newTestBitbucketPR(5, "PR without avatar", "OPEN", "fix",
		"2026-01-01T00:00:00.000000+00:00", "2026-01-01T00:00:00.000000+00:00")
	bbPR.Author.DisplayName = "No Avatar User"
	bbPR.Author.UUID = "{no-avatar-uuid}"
	// Avatar Href is empty string (zero value)

	prResponse := bitbucketPRResponse{
		Size:    1,
		Page:    1,
		Pagelen: 20,
		Values:  []bitbucketPR{bbPR},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		body, _ := json.Marshal(prResponse)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}
	token := testBitbucketToken()

	result, err := svc.ListPullRequests(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: token},
		models.PullRequestListOptions{State: "open", Page: 1, PerPage: 20},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)

	pr := result.Data[0]
	require.NotNil(t, pr.Author)
	assert.Equal(t, "No Avatar User", pr.Author.Name)
	assert.Nil(t, pr.Author.AvatarUrl, "avatar_url should be nil when empty")
}

func TestBitbucketService_ListPullRequests_InvalidToken(t *testing.T) {
	svc := NewBitbucketProvider()

	_, err := svc.ListPullRequests(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: "not-valid-base64!!!"},
		models.PullRequestListOptions{State: "open", Page: 1, PerPage: 20},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode bitbucket token")
}

func TestBitbucketService_ListPullRequests_InvalidTimestamp(t *testing.T) {
	prResponse := bitbucketPRResponse{
		Size:    1,
		Page:    1,
		Pagelen: 20,
		Values: []bitbucketPR{
			newTestBitbucketPR(1, "Bad timestamp PR", "OPEN", "fix",
				"not-a-timestamp", "2026-01-01T00:00:00.000000+00:00"),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		body, _ := json.Marshal(prResponse)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}
	token := testBitbucketToken()

	_, err := svc.ListPullRequests(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: token},
		models.PullRequestListOptions{State: "open", Page: 1, PerPage: 20},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse created_on time")
}

func TestBitbucketService_ListPullRequests_PaginationParams(t *testing.T) {
	var capturedReq *http.Request

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := bitbucketPRResponse{Size: 0, Page: 3, Pagelen: 10, Values: []bitbucketPR{}}
		body, _ := json.Marshal(resp)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}
	token := testBitbucketToken()

	_, err := svc.ListPullRequests(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: token},
		models.PullRequestListOptions{State: "open", Page: 3, PerPage: 10},
	)

	require.NoError(t, err)
	require.NotNil(t, capturedReq)

	assert.Equal(t, "3", capturedReq.URL.Query().Get("page"))
	assert.Equal(t, "10", capturedReq.URL.Query().Get("pagelen"))
}

func TestBitbucketService_ListPullRequests_SupersededState(t *testing.T) {
	prResponse := bitbucketPRResponse{
		Size:    2,
		Page:    1,
		Pagelen: 20,
		Values: []bitbucketPR{
			newTestBitbucketPR(10, "Superseded PR", "SUPERSEDED", "old-branch",
				"2026-01-01T00:00:00.000000+00:00", "2026-01-02T00:00:00.000000+00:00"),
			newTestBitbucketPR(11, "Declined PR", "DECLINED", "rejected-branch",
				"2026-01-01T00:00:00.000000+00:00", "2026-01-02T00:00:00.000000+00:00"),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		body, _ := json.Marshal(prResponse)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}
	token := testBitbucketToken()

	result, err := svc.ListPullRequests(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{Token: token},
		models.PullRequestListOptions{State: "all", Page: 1, PerPage: 20},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 2)

	// Both SUPERSEDED and DECLINED should map to closed
	assert.Equal(t, models.PullRequestStateClosed, result.Data[0].State)
	assert.Equal(t, models.PullRequestStateClosed, result.Data[1].State)
}
