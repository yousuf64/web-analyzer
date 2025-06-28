package analyzer

import (
	"analyzer/internal/config"
	"log/slog"
	"net/http"
	"shared/messagebus"
	"shared/metrics"
	"shared/repository"
	"time"
)

// Analyzer handles HTML analysis with all dependencies consolidated
type Analyzer struct {
	jobRepo   *repository.JobRepository
	taskRepo  *repository.TaskRepository
	publisher *messagebus.MessageBus
	client    *http.Client
	metrics   metrics.AnalyzerMetricsInterface
	log       *slog.Logger
	cfg       *config.Config
}

// AnalysisResult holds the internal analysis results
type AnalysisResult struct {
	htmlVersion       string
	title             string
	headings          map[string]int
	links             []string
	internalLinks     int32
	externalLinks     int32
	accessibleLinks   int32
	inaccessibleLinks int32
	hasLoginForm      bool
	baseURL           string
}

// Option configures the Analyzer
type Option func(*Analyzer)

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(s *Analyzer) {
		s.client = client
	}
}

// WithMetrics sets the metrics collector
func WithMetrics(metrics metrics.AnalyzerMetricsInterface) Option {
	return func(s *Analyzer) {
		s.metrics = metrics
	}
}

// WithLogger sets the logger
func WithLogger(log *slog.Logger) Option {
	return func(s *Analyzer) {
		s.log = log
	}
}

// WithConfig sets the configuration
func WithConfig(cfg *config.Config) Option {
	return func(s *Analyzer) {
		s.cfg = cfg
	}
}

// NewAnalyzer creates a new analyzer with required dependencies and optional configurations
func NewAnalyzer(
	jobRepo *repository.JobRepository,
	taskRepo *repository.TaskRepository,
	publisher *messagebus.MessageBus,
	opts ...Option,
) *Analyzer {
	s := &Analyzer{
		jobRepo:   jobRepo,
		taskRepo:  taskRepo,
		publisher: publisher,
		client:    &http.Client{Timeout: 20 * time.Second},
		metrics:   metrics.NewNoOpAnalyzerMetrics(),
		log:       slog.Default(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}
