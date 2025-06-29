package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"shared/middleware"
	"shared/mocks"
	"shared/models"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yousuf64/shift"
	"go.uber.org/mock/gomock"
)

// handlerTestCase is a test case for API handler testing
type handlerTestCase struct {
	name           string
	method         string
	path           string
	body           any
	setupMocks     func(*mocks.MockJobRepositoryInterface, *mocks.MockTaskRepositoryInterface, *mocks.MockMessageBusInterface)
	expectedStatus int
	expectedError  bool
	description    string
}

// setupMockAPI creates an API instance with mocked dependencies
func setupMockAPI(t *testing.T) (*API, *mocks.MockJobRepositoryInterface, *mocks.MockTaskRepositoryInterface, *mocks.MockMessageBusInterface, *gomock.Controller) {
	ctrl := gomock.NewController(t)

	mockJobRepo := mocks.NewMockJobRepositoryInterface(ctrl)
	mockTaskRepo := mocks.NewMockTaskRepositoryInterface(ctrl)
	mockMessageBus := mocks.NewMockMessageBusInterface(ctrl)

	// Create API with interfaces for testing
	api := &API{
		jobRepo:  mockJobRepo,
		taskRepo: mockTaskRepo,
		mb:       mockMessageBus,
		metrics:  nil,
		log:      slog.New(slog.DiscardHandler),
	}

	return api, mockJobRepo, mockTaskRepo, mockMessageBus, ctrl
}

// makeRequest creates an HTTP request with the given method, path, and body.
func makeRequest(method, path string, body any) (*http.Request, error) {
	var reqBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&reqBody).Encode(body); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, path, &reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// setupRouter creates a new router and registers the given handler for the given method and path.
// It also adds the error middleware to the router.
func setupRouter(method, path string, handler shift.HandlerFunc) *shift.Router {
	router := shift.New()
	router.Use(middleware.ErrorMiddleware(slog.New(slog.DiscardHandler)))
	router.Map([]string{method}, path, handler)
	return router
}

func TestAPI_HandleAnalyze_TableDriven(t *testing.T) {
	testCases := []handlerTestCase{
		// Success cases
		{
			name:   "SuccessfulAnalyze_HTTPS",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://example.com",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				jobRepo.EXPECT().CreateJob(gomock.Any(), gomock.Any()).Return(nil)
				taskRepo.EXPECT().CreateTasks(gomock.Any(), gomock.Any()).Return(nil)
				mb.EXPECT().PublishAnalyzeMessage(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedStatus: http.StatusAccepted,
			expectedError:  false,
			description:    "Successfully create and publish analysis job with HTTPS URL",
		},
		{
			name:   "SuccessfulAnalyze_HTTP",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "http://example.com",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				jobRepo.EXPECT().CreateJob(gomock.Any(), gomock.Any()).Return(nil)
				taskRepo.EXPECT().CreateTasks(gomock.Any(), gomock.Any()).Return(nil)
				mb.EXPECT().PublishAnalyzeMessage(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedStatus: http.StatusAccepted,
			expectedError:  false,
			description:    "Successfully create and publish analysis job with HTTP URL",
		},
		{
			name:   "SuccessfulAnalyze_NoScheme",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "example.com",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				jobRepo.EXPECT().CreateJob(gomock.Any(), gomock.Any()).Return(nil)
				taskRepo.EXPECT().CreateTasks(gomock.Any(), gomock.Any()).Return(nil)
				mb.EXPECT().PublishAnalyzeMessage(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedStatus: http.StatusAccepted,
			expectedError:  false,
			description:    "Successfully create job with URL without scheme (auto-adds https://)",
		},
		{
			name:   "SuccessfulAnalyze_WithPath",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://example.com/path/to/page",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				jobRepo.EXPECT().CreateJob(gomock.Any(), gomock.Any()).Return(nil)
				taskRepo.EXPECT().CreateTasks(gomock.Any(), gomock.Any()).Return(nil)
				mb.EXPECT().PublishAnalyzeMessage(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedStatus: http.StatusAccepted,
			expectedError:  false,
			description:    "Successfully create job with URL containing path",
		},

		// URL validation error cases
		{
			name:   "EmptyURL",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject empty URL",
		},
		{
			name:   "WhitespaceOnlyURL",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "   ",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject whitespace-only URL",
		},
		{
			name:   "TooLongURL",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://example.com/" + strings.Repeat("a", 2050), // Over 2048 character limit
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject URL exceeding 2048 character limit",
		},
		{
			name:   "UnsupportedScheme_FTP",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "ftp://example.com",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject FTP scheme (only HTTP/HTTPS allowed)",
		},
		{
			name:   "UnsupportedScheme_File",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "file:///etc/passwd",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject file scheme (security risk)",
		},
		{
			name:   "MissingHostname",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject URL without hostname",
		},
		{
			name:   "LocalhostRejection",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://localhost",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject localhost URLs (security policy)",
		},
		{
			name:   "LoopbackIP_127001",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://127.0.0.1",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject loopback IP address 127.0.0.1",
		},
		{
			name:   "LoopbackIP_IPv6",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://[::1]",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject IPv6 loopback address ::1",
		},
		{
			name:   "PrivateIP_192168",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://192.168.1.1",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject private IP address 192.168.x.x",
		},
		{
			name:   "PrivateIP_10x",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://10.0.0.1",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject private IP address 10.x.x.x",
		},
		{
			name:   "PrivateIP_172x",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://172.16.0.1",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject private IP address 172.16.x.x",
		},
		{
			name:   "PathTraversalAttack",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://example.com/../../../etc/passwd",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject URLs with path traversal patterns (..)",
		},
		{
			name:   "InvalidHostnameFormat",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://invalid..hostname",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject invalid hostname format (double dots)",
		},
		{
			name:   "LocalhostSubdomain",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://test.localhost",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject .localhost subdomains",
		},
		{
			name:   "EmptyHostname_WithPort",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://:8080",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail validation
			},
			expectedError: true,
			description:   "Reject empty hostname with port",
		},

		// JSON and request parsing errors
		{
			name:   "InvalidJSON",
			method: "POST",
			path:   "/analyze",
			body:   "invalid-json",
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - should fail JSON parsing
			},
			expectedError: true,
			description:   "Handle invalid JSON request body",
		},

		// Database and infrastructure errors
		{
			name:   "DatabaseError",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://example.com",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				jobRepo.EXPECT().CreateJob(gomock.Any(), gomock.Any()).Return(errors.New("database error"))
			},
			expectedError: true,
			description:   "Handle database errors during job creation",
		},
		{
			name:   "TaskCreationError",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://example.com",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				jobRepo.EXPECT().CreateJob(gomock.Any(), gomock.Any()).Return(nil)
				taskRepo.EXPECT().CreateTasks(gomock.Any(), gomock.Any()).Return(errors.New("task creation failed"))
			},
			expectedError: true,
			description:   "Handle task creation errors",
		},
		{
			name:   "MessageBusError",
			method: "POST",
			path:   "/analyze",
			body: AnalyzeRequest{
				URL: "https://example.com",
			},
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				jobRepo.EXPECT().CreateJob(gomock.Any(), gomock.Any()).Return(nil)
				taskRepo.EXPECT().CreateTasks(gomock.Any(), gomock.Any()).Return(nil)
				mb.EXPECT().PublishAnalyzeMessage(gomock.Any(), gomock.Any()).Return(errors.New("message bus error"))
			},
			expectedError: true,
			description:   "Handle message bus publishing errors",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			api, mockJobRepo, mockTaskRepo, mockMessageBus, ctrl := setupMockAPI(t)
			defer ctrl.Finish()

			// Configure mocks
			tc.setupMocks(mockJobRepo, mockTaskRepo, mockMessageBus)

			// Create request
			req, err := makeRequest(tc.method, tc.path, tc.body)
			assert.NoError(t, err, "Failed to create request")

			// Create response recorder
			rr := httptest.NewRecorder()

			// Create router and register route
			router := setupRouter("POST", "/analyze", api.handleAnalyze)

			// Act
			router.Serve().ServeHTTP(rr, req)

			// Assert
			if tc.expectedError {
				assert.True(t, rr.Code >= 400, "Expected error status code, got %d", rr.Code)
			} else {
				assert.Equal(t, tc.expectedStatus, rr.Code, "Status code mismatch")
			}
		})
	}
}

func TestAPI_HandleGetJobs_TableDriven(t *testing.T) {
	testJobs := []*models.Job{
		{
			ID:        "job-1",
			URL:       "https://example.com",
			Status:    models.JobStatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "job-2",
			URL:       "https://test.com",
			Status:    models.JobStatusRunning,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	testCases := []handlerTestCase{
		{
			name:   "SuccessfulGetJobs",
			method: "GET",
			path:   "/jobs",
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				jobRepo.EXPECT().GetAllJobs(gomock.Any()).Return(testJobs, nil)
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
			description:    "Successfully retrieve all jobs",
		},
		{
			name:   "EmptyJobsList",
			method: "GET",
			path:   "/jobs",
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				jobRepo.EXPECT().GetAllJobs(gomock.Any()).Return([]*models.Job{}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
			description:    "Handle empty jobs list",
		},
		{
			name:   "DatabaseError",
			method: "GET",
			path:   "/jobs",
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				jobRepo.EXPECT().GetAllJobs(gomock.Any()).Return(nil, errors.New("database error"))
			},
			expectedError: true,
			description:   "Handle database errors when fetching jobs",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			api, mockJobRepo, mockTaskRepo, mockMessageBus, ctrl := setupMockAPI(t)
			defer ctrl.Finish()

			// Configure mocks
			tc.setupMocks(mockJobRepo, mockTaskRepo, mockMessageBus)

			// Create request
			req, err := makeRequest(tc.method, tc.path, tc.body)
			assert.NoError(t, err, "Failed to create request")

			// Create response recorder
			rr := httptest.NewRecorder()

			// Create router and register route
			router := setupRouter("GET", "/jobs", api.handleGetJobs)

			// Act
			router.Serve().ServeHTTP(rr, req)

			// Assert
			if tc.expectedError {
				assert.True(t, rr.Code >= 400, "Expected error status code, got %d", rr.Code)
			} else {
				assert.Equal(t, tc.expectedStatus, rr.Code, "Status code mismatch")
				if tc.expectedStatus == http.StatusOK {
					var responseJobs []*models.Job
					err := json.Unmarshal(rr.Body.Bytes(), &responseJobs)
					assert.NoError(t, err, "Response should be valid JSON")
				}
			}
		})
	}
}

func TestAPI_HandleGetTasksByJobID_TableDriven(t *testing.T) {
	testTasks := []models.Task{
		{
			JobID:  "job-1",
			Type:   models.TaskTypeExtracting,
			Status: models.TaskStatusCompleted,
		},
		{
			JobID:  "job-1",
			Type:   models.TaskTypeAnalyzing,
			Status: models.TaskStatusRunning,
		},
	}

	testCases := []struct {
		name           string
		jobID          string
		setupMocks     func(*mocks.MockJobRepositoryInterface, *mocks.MockTaskRepositoryInterface, *mocks.MockMessageBusInterface)
		expectedStatus int
		expectedError  bool
		description    string
	}{
		{
			name:  "SuccessfulGetTasks",
			jobID: "job-1",
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				taskRepo.EXPECT().GetTasksByJobId(gomock.Any(), "job-1").Return(testTasks, nil)
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
			description:    "Successfully retrieve tasks for a job",
		},
		{
			name:  "EmptyTasksList",
			jobID: "job-2",
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				taskRepo.EXPECT().GetTasksByJobId(gomock.Any(), "job-2").Return([]models.Task{}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
			description:    "Handle empty tasks list",
		},
		{
			name:  "DatabaseError",
			jobID: "job-3",
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				taskRepo.EXPECT().GetTasksByJobId(gomock.Any(), "job-3").Return(nil, errors.New("database error"))
			},
			expectedError: true,
			description:   "Handle database errors when fetching tasks",
		},
		{
			name:  "MissingJobID",
			jobID: "", // Empty job ID to test validation
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - shift would not match the route
			},
			expectedError: true,
			description:   "Handle missing job_id parameter",
		},
		{
			name:  "MissingJobID_Space",
			jobID: " ", // Empty job ID with space to test validation
			setupMocks: func(jobRepo *mocks.MockJobRepositoryInterface, taskRepo *mocks.MockTaskRepositoryInterface, mb *mocks.MockMessageBusInterface) {
				// No expectations - would not reach the repository
			},
			expectedError: true,
			description:   "Handle missing job_id parameter",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			api, mockJobRepo, mockTaskRepo, mockMessageBus, ctrl := setupMockAPI(t)
			defer ctrl.Finish()

			// Configure mocks
			tc.setupMocks(mockJobRepo, mockTaskRepo, mockMessageBus)

			// Create request with proper job ID in URL
			url := "/jobs/" + tc.jobID + "/tasks"
			req, err := makeRequest("GET", url, nil)
			assert.NoError(t, err, "Failed to create request")

			// Create response recorder
			rr := httptest.NewRecorder()

			// Create router and register route with job_id parameter
			router := setupRouter("GET", "/jobs/:job_id/tasks", api.handleGetTasksByJobID)

			// Act
			router.Serve().ServeHTTP(rr, req)

			// Assert
			if tc.expectedError {
				assert.True(t, rr.Code >= 400, "Expected error status code, got %d", rr.Code)
			} else {
				assert.Equal(t, tc.expectedStatus, rr.Code, "Status code mismatch")
				if tc.expectedStatus == http.StatusOK {
					var responseTasks []models.Task
					err := json.Unmarshal(rr.Body.Bytes(), &responseTasks)
					assert.NoError(t, err, "Response should be valid JSON")
				}
			}
		})
	}
}
