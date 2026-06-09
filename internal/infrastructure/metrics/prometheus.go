package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Define HTTP Metrics
var (
	APIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_ms",
			Help:    "Thời gian xử lý REST API request từ Client.",
			Buckets: []float64{5, 10, 25, 50, 100, 250, 500, 1000},
		},
		[]string{"method", "endpoint", "status_code"},
	)

	APIRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_request_total",
			Help: "Tổng số lượng REST API request.",
		},
		[]string{"method", "endpoint", "status_code"},
	)
)

// Define gRPC Metrics
var (
	GRPCRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_ms",
			Help:    "Thời gian gọi gRPC sang AI Server.",
			Buckets: []float64{5, 10, 25, 50, 100, 250, 500, 1000},
		},
		[]string{"method", "status_code"},
	)

	GRPCRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_request_total",
			Help: "Tổng số lượng gRPC request.",
		},
		[]string{"method", "status_code"},
	)
)

// Define Cache Metrics
var (
	RedisCacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_cache_hits_total",
			Help: "Đếm số lần cache hit thành công trên Redis.",
		},
		[]string{"cache_type"},
	)

	RedisCacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_cache_misses_total",
			Help: "Đếm số lần cache miss trên Redis.",
		},
		[]string{"cache_type"},
	)

	// User Event Metrics
	FeedbackEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feedback_received_total",
			Help: "Total number of user feedback actions submitted (correction, acceptance, viewed)",
		},
		[]string{"action"},
	)

	PlannerGenerationFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "planner_generation_failures_total",
			Help: "Total planner generation failures",
		},
		[]string{"reason"},
	)

	PlannerGenerationDurationMs = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "planner_generation_duration_ms",
			Help:    "Thời gian chạy pipeline tạo Weekly Plan.",
			Buckets: []float64{50, 100, 250, 500, 1000, 2000, 5000},
		},
		[]string{"method", "status_code"},
	)
)
