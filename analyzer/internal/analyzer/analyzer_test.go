package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"shared/messagebus"
	"shared/mocks"
	"shared/models"
	"strings"
	"sync"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

const shouldNotBeFound = "should_not_be_found"
const shouldRetryAndFail = "should_retry_and_fail"

// MockHTTPRoundTripper implements http.RoundTripper for testing
type MockHTTPRoundTripper struct {
	statusCode  int
	htmlContent string
}

func (m *MockHTTPRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.String(), shouldNotBeFound) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}

	if strings.Contains(req.URL.String(), shouldRetryAndFail) {
		return &http.Response{
			StatusCode: http.StatusMethodNotAllowed,
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}

	return &http.Response{
		StatusCode: m.statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader([]byte(m.htmlContent))),
		Request:    req,
	}, nil
}

// TestCase represents a single test scenario
type TestCase struct {
	name                 string
	htmlFile             string
	testURL              string
	expectedTitle        string
	expectedHTMLVersion  string
	expectedHeadings     map[string]int
	expectedExternal     int
	expectedInternal     int
	expectedAccessible   int
	expectedInaccessible int
	expectedLoginForm    bool
	description          string
}

// SubTaskCapture stores captured subtask information
type SubTaskCapture struct {
	JobID    string
	TaskType models.TaskType
	Key      string
	SubTask  models.SubTask
}

// setupMockAnalyzer creates a new analyzer with mocked dependencies and subtask tracking
func setupMockAnalyzer(t *testing.T, htmlContent string, testURL string) (*Analyzer, **models.AnalyzeResult, *gomock.Controller, *[]SubTaskCapture) {
	ctrl := gomock.NewController(t)

	mockJobRepo := mocks.NewMockJobRepositoryInterface(ctrl)
	mockTaskRepo := mocks.NewMockTaskRepositoryInterface(ctrl)
	mockMessageBus := mocks.NewMockMessageBusInterface(ctrl)

	// Variable to capture the analysis result
	var capturedResult *models.AnalyzeResult

	// Slice to capture all subtask operations
	var capturedSubTasks []SubTaskCapture
	var captureLock sync.Mutex

	// Create mock HTTP client
	mockTransport := &MockHTTPRoundTripper{statusCode: 200, htmlContent: htmlContent}
	mockHTTPClient := &http.Client{Transport: mockTransport}

	// Set up mock expectations
	mockJobRepo.EXPECT().GetJob(gomock.Any(), gomock.Any()).Return(&models.Job{
		ID:     "test-job-id",
		URL:    testURL,
		Status: models.JobStatusPending,
	}, nil).AnyTimes()

	mockJobRepo.EXPECT().UpdateJob(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, jobID string, status *models.JobStatus, result *models.AnalyzeResult) error {
			capturedResult = result
			return nil
		}).AnyTimes()

	mockJobRepo.EXPECT().UpdateJobStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockTaskRepo.EXPECT().UpdateTaskStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	// Capture AddSubTaskByKey calls
	mockTaskRepo.EXPECT().AddSubTaskByKey(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, jobID string, taskType models.TaskType, key string, subtask models.SubTask) error {
			captureLock.Lock()
			defer captureLock.Unlock()
			capturedSubTasks = append(capturedSubTasks, SubTaskCapture{
				JobID:    jobID,
				TaskType: taskType,
				Key:      key,
				SubTask:  subtask,
			})
			return nil
		}).AnyTimes()

	// Capture UpdateSubTaskByKey calls
	mockTaskRepo.EXPECT().UpdateSubTaskByKey(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, jobID string, taskType models.TaskType, key string, subtask models.SubTask) error {
			captureLock.Lock()
			defer captureLock.Unlock()
			capturedSubTasks = append(capturedSubTasks, SubTaskCapture{
				JobID:    jobID,
				TaskType: taskType,
				Key:      key,
				SubTask:  subtask,
			})
			return nil
		}).AnyTimes()

	mockMessageBus.EXPECT().PublishJobUpdate(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockMessageBus.EXPECT().PublishTaskStatusUpdate(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockMessageBus.EXPECT().PublishSubTaskUpdate(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	analyzer := NewAnalyzer(
		mockJobRepo,
		mockTaskRepo,
		mockMessageBus,
		WithHTTPClient(mockHTTPClient),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	return analyzer, &capturedResult, ctrl, &capturedSubTasks
}

func TestAnalyzer_HTMLAnalysis(t *testing.T) {
	testCases := []TestCase{
		{
			name:                "SimpleBlog",
			htmlFile:            "testdata/simple_blog.html",
			testURL:             "https://blog.example.com",
			expectedTitle:       "Simple Blog - Web Development Tips",
			expectedHTMLVersion: "HTML5",
			expectedHeadings: map[string]int{
				"h1": 1, // "Simple Blog"
				"h2": 2, // "Getting Started with Go", "HTML Best Practices"
				"h3": 2, // "Quick Links", "Login"
			},
			expectedExternal:     2, // GitHub, Golang.org
			expectedInternal:     6, // /, /about, /contact, /archives, /tags, /privacy, /terms
			expectedAccessible:   8,
			expectedInaccessible: 0,
			expectedLoginForm:    true,
			description:          "Blog with mixed content, login form, and various link types",
		},
		{
			name:                "EmptyPage",
			htmlFile:            "testdata/empty_page.html",
			testURL:             "https://minimal.example.com",
			expectedTitle:       "Empty Page",
			expectedHTMLVersion: "HTML5",
			expectedHeadings: map[string]int{
				"h1": 1, // "Nothing Here"
			},
			expectedExternal:     0, // No external links
			expectedInternal:     0, // No internal links
			expectedAccessible:   0,
			expectedInaccessible: 0,
			expectedLoginForm:    false,
			description:          "Minimal page with no links or forms - testing edge cases",
		},
		{
			name:                "ComplexEcommerce",
			htmlFile:            "testdata/complex_site.html",
			testURL:             "https://shop.megastore.com",
			expectedTitle:       "Complex E-commerce Site",
			expectedHTMLVersion: "HTML5",
			expectedHeadings: map[string]int{
				"h1": 1, // "MegaStore"
				"h2": 2, // "Featured Products", "Categories"
				"h3": 4, // "Laptop Computer", "Smartphone", "Electronics", "Accessories"
				"h4": 2, // "Customer Login", "Newsletter"
				"h5": 1, // "Quick Links"
			},
			expectedExternal:     6, // support.megastore.com, facebook.com, twitter.com, paypal.com, stripe.com, ups.com
			expectedInternal:     17,
			expectedAccessible:   21,
			expectedInaccessible: 2,
			expectedLoginForm:    true,
			description:          "Complex e-commerce site with multiple forms, many links, and deep heading hierarchy",
		},
		{
			name:                "OldHTML",
			htmlFile:            "testdata/old_html.html",
			testURL:             "https://retro.geocities.com/site",
			expectedTitle:       "Old HTML Page",
			expectedHTMLVersion: "HTML 4.01 Strict",
			expectedHeadings: map[string]int{
				"h1": 1, // "Retro Website"
				"h2": 1, // "Welcome to the 90s!"
			},
			expectedExternal:     1, // http://example.com/external
			expectedInternal:     2, // /page1.html, /page2.html
			expectedAccessible:   3,
			expectedInaccessible: 0,
			expectedLoginForm:    false,
			description:          "Old HTML 4.01 page with table layout and basic form - testing legacy HTML detection",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Read the HTML file
			htmlContent, err := os.ReadFile(tc.htmlFile)
			assert.NoError(t, err, "Failed to read HTML file: %s", tc.htmlFile)

			// Setup analyzer with mocks
			analyzer, capturedResult, ctrl, capturedSubTasks := setupMockAnalyzer(t, string(htmlContent), tc.testURL)
			defer ctrl.Finish()

			msg, err := json.Marshal(messagebus.AnalyzeMessage{
				JobId: "test-job-id",
			})
			assert.NoError(t, err, "Failed to marshal analyze message")

			// Execute the analysis
			analyzer.ProcessAnalyzeMessage(context.Background(), &nats.Msg{
				Data:    msg,
				Subject: "url.analyze",
			})

			assert.NotNil(t, *capturedResult, "Analysis result should not be nil")
			result := *capturedResult

			// Verify core analysis results
			assert.Equal(t, tc.expectedHTMLVersion, result.HtmlVersion, "HTML version mismatch")
			assert.Equal(t, tc.expectedTitle, result.PageTitle, "Page title mismatch")
			assert.Equal(t, tc.expectedHeadings, result.Headings, "Headings count mismatch")
			assert.Equal(t, tc.expectedExternal, result.ExternalLinkCount, "External links count mismatch")
			assert.Equal(t, tc.expectedInternal, result.InternalLinkCount, "Internal links count mismatch")
			assert.Equal(t, tc.expectedAccessible, result.AccessibleLinks, "Accessible links count mismatch")
			assert.Equal(t, tc.expectedInaccessible, result.InaccessibleLinks, "Inaccessible links count mismatch")
			assert.Equal(t, tc.expectedLoginForm, result.HasLoginForm, "Login form detection mismatch")

			totalExpectedLinks := tc.expectedExternal + tc.expectedInternal
			if totalExpectedLinks > 0 {
				assert.NotEmpty(t, result.Links, "Should find links in the HTML")
			} else {
				assert.Empty(t, result.Links, "Should not find links in this HTML")
			}

			assert.Equal(t, result.InternalLinkCount+result.ExternalLinkCount, len(result.Links),
				"Total links should equal internal + external")

			subtasksByURL := make(map[string]SubTaskCapture)
			for _, subtask := range *capturedSubTasks {
				subtasksByURL[subtask.SubTask.URL] = subtask
			}

			// Verify subtasks for each link
			for _, link := range result.Links {
				subtask, ok := subtasksByURL[link]
				if !ok {
					t.Errorf("Subtask not found for link %s", link)
					continue
				}

				assert.Equal(t, subtask.JobID, "test-job-id", "JobID should match")
				assert.Equal(t, subtask.TaskType, models.TaskTypeVerifyingLinks, "TaskType should match")
				assert.Equal(t, subtask.SubTask.Type, models.SubTaskTypeValidatingLink, "SubTaskType should match")
				assert.Equal(t, subtask.SubTask.URL, link, "URL should match")

				if strings.Contains(link, shouldNotBeFound) {
					assert.Equal(t, subtask.SubTask.Status, models.TaskStatusFailed, "Subtask should be failed")
					assert.Equal(t, subtask.SubTask.Description, "HTTP 404: Not Found", "Subtask should have correct description")
				} else if strings.Contains(link, shouldRetryAndFail) {
					assert.Equal(t, subtask.SubTask.Status, models.TaskStatusFailed, "Subtask should be failed")
					assert.Equal(t, subtask.SubTask.Description, "HTTP 405: Method Not Allowed", "Subtask should have correct description")
				} else {
					assert.Equal(t, subtask.SubTask.Status, models.TaskStatusCompleted, "Subtask should be completed")
				}
			}
		})
	}
}

func TestAnalyzer_JobNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJobRepo := mocks.NewMockJobRepositoryInterface(ctrl)
	mockTaskRepo := mocks.NewMockTaskRepositoryInterface(ctrl)
	mockMessageBus := mocks.NewMockMessageBusInterface(ctrl)

	mockJobRepo.EXPECT().GetJob(gomock.Any(), gomock.Any()).Return(nil, errors.New("job not found"))

	// Should still attempt to update the job status and task statuses
	mockJobRepo.EXPECT().UpdateJobStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockTaskRepo.EXPECT().UpdateTaskStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(4)
	mockMessageBus.EXPECT().PublishJobUpdate(gomock.Any(), gomock.Any()).Return(nil)
	mockMessageBus.EXPECT().PublishTaskStatusUpdate(gomock.Any(), gomock.Any()).Return(nil).Times(4)

	analyzer := NewAnalyzer(
		mockJobRepo,
		mockTaskRepo,
		mockMessageBus,
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	msg, err := json.Marshal(messagebus.AnalyzeMessage{
		JobId: "test-job-id",
	})
	assert.NoError(t, err, "Failed to marshal analyze message")

	analyzer.ProcessAnalyzeMessage(context.Background(), &nats.Msg{
		Data:    msg,
		Subject: "url.analyze",
	})
}

func TestAnalyzer_FailedToMarshalAnalyzeMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJobRepo := mocks.NewMockJobRepositoryInterface(ctrl)
	mockTaskRepo := mocks.NewMockTaskRepositoryInterface(ctrl)
	mockMessageBus := mocks.NewMockMessageBusInterface(ctrl)

	// Should not attempt to get the job and exit early
	mockJobRepo.EXPECT().GetJob(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)

	analyzer := NewAnalyzer(
		mockJobRepo,
		mockTaskRepo,
		mockMessageBus,
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	analyzer.ProcessAnalyzeMessage(context.Background(), &nats.Msg{
		Data:    []byte(`invalid`),
		Subject: "url.analyze",
	})
}

func TestAnalyzer_FailedToFetchContent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJobRepo := mocks.NewMockJobRepositoryInterface(ctrl)
	mockTaskRepo := mocks.NewMockTaskRepositoryInterface(ctrl)
	mockMessageBus := mocks.NewMockMessageBusInterface(ctrl)

	mockJobRepo.EXPECT().GetJob(gomock.Any(), gomock.Any()).Return(&models.Job{
		ID:     "test-job-id",
		URL:    "https://www.google.com",
		Status: models.JobStatusPending,
	}, nil)

	var capturedJobStatus models.JobStatus
	mockJobRepo.EXPECT().UpdateJobStatus(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, jobID string, status models.JobStatus) error {
		capturedJobStatus = status
		return nil
	}).AnyTimes()
	mockJobRepo.EXPECT().UpdateJob(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockTaskRepo.EXPECT().UpdateTaskStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockMessageBus.EXPECT().PublishJobUpdate(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockMessageBus.EXPECT().PublishTaskStatusUpdate(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	mockHTTPClient := &http.Client{
		Transport: &MockHTTPRoundTripper{
			statusCode:  http.StatusBadRequest,
			htmlContent: "",
		},
	}

	analyzer := NewAnalyzer(
		mockJobRepo,
		mockTaskRepo,
		mockMessageBus,
		WithHTTPClient(mockHTTPClient),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	msg, err := json.Marshal(messagebus.AnalyzeMessage{
		JobId: "test-job-id",
	})
	assert.NoError(t, err, "Failed to marshal analyze message")

	analyzer.ProcessAnalyzeMessage(context.Background(), &nats.Msg{
		Data:    msg,
		Subject: "url.analyze",
	})

	assert.Equal(t, models.JobStatusFailed, capturedJobStatus, "Job status should be failed")
}
