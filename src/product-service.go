package main

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ---------- Observability: RED metrics ----------

var (
	// Rate: every request, labelled by route template and status code
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "route", "status"},
	)

	// Errors: only non-2xx responses
	httpRequestErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_request_errors_total",
			Help: "Total number of HTTP requests with a non-2xx status.",
		},
		[]string{"method", "route", "status"},
	)

	// Duration: latency histogram, used for p50/p95 in Grafana
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"method", "route"},
	)
)

// Structured JSON logger: one line per request to stdout
var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// observability records metrics and emits one JSON log line per request.
func observability() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Don't measure Prometheus scrapes of /metrics itself
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()
		c.Next() // run the actual handler

		duration := time.Since(start)
		status := c.Writer.Status()
		statusStr := strconv.Itoa(status)
		method := c.Request.Method

		// Route template (/products/:productId), not the raw URL,
		// so each product ID doesn't create a new time series.
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		httpRequestsTotal.WithLabelValues(method, route, statusStr).Inc()
		if status < 200 || status >= 300 {
			httpRequestErrorsTotal.WithLabelValues(method, route, statusStr).Inc()
		}
		httpRequestDuration.WithLabelValues(method, route).Observe(duration.Seconds())

		logger.Info("request",
			"endpoint", route,
			"path", c.Request.URL.Path,
			"method", method,
			"status", status,
			"duration_ms", float64(duration.Microseconds())/1000.0,
		)
	}
}

// ---------- Product API ----------

type Product struct {
	ProductID    int32  `json:"product_id"    binding:"required,min=1"`
	SKU          string `json:"sku"           binding:"required,min=1,max=100"`
	Manufacturer string `json:"manufacturer"  binding:"required,min=1,max=200"`
	CategoryID   int32  `json:"category_id"   binding:"required,min=1"`
	Weight       *int32 `json:"weight"        binding:"required,min=0"`
	SomeOtherID  int32  `json:"some_other_id" binding:"required,min=1"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type ProductStore struct {
	mu       sync.RWMutex
	products map[int32]Product
}

func NewProductStore() *ProductStore {
	return &ProductStore{products: make(map[int32]Product)}
}

func (s *ProductStore) Get(id int32) (Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.products[id]
	return p, ok
}

func (s *ProductStore) Exists(id int32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.products[id]
	return ok
}

func (s *ProductStore) Save(p Product) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.products[p.ProductID] = p
}

func getProduct(store *ProductStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-Simulate-Error") == "true" {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
			})
			return
		}

		idStr := c.Param("productId")

		id, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil || id < 1 {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Product not found",
				Details: "Product ID must be a positive 32-bit integer",
			})
			return
		}

		product, ok := store.Get(int32(id))
		if !ok {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Product not found",
			})
			return
		}

		c.JSON(http.StatusOK, product)
	}
}

func addProductDetails(store *ProductStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-Simulate-Error") == "true" {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
			})
			return
		}

		idStr := c.Param("productId")

		id, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil || id < 1 {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "Invalid product ID",
				Details: "Product ID must be a positive 32-bit integer",
			})
			return
		}

		var p Product
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: err.Error(),
			})
			return
		}

		if p.ProductID != int32(id) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "Product ID mismatch",
				Details: "product_id in body must match productId in path",
			})
			return
		}

		if !store.Exists(int32(id)) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Product not found",
			})
			return
		}

		store.Save(p)
		c.Status(http.StatusNoContent)
	}
}

func main() {
	store := NewProductStore()

	// Seed starting products
	w1, w2, w3 := int32(1250), int32(500), int32(0)
	store.Save(Product{ProductID: 1, SKU: "ABC-123", Manufacturer: "Acme", CategoryID: 10, Weight: &w1, SomeOtherID: 100})
	store.Save(Product{ProductID: 2, SKU: "DEF-456", Manufacturer: "Globex", CategoryID: 20, Weight: &w2, SomeOtherID: 200})
	store.Save(Product{ProductID: 3, SKU: "GHI-789", Manufacturer: "Initech", CategoryID: 30, Weight: &w3, SomeOtherID: 300})

	// gin.New() instead of gin.Default(): drops Gin's text logger,
	// so each request produces only our one JSON log line.
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(observability())

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/products/:productId", getProduct(store))
	r.POST("/products/:productId/details", addProductDetails(store))

	r.Run(":8080")
}