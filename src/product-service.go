package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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
			Buckets: []float64{0.00005, 0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},		},
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

		attrs := []any{
			"endpoint", route,
			"path", c.Request.URL.Path,
			"method", method,
			"status", status,
			"duration_ms", float64(duration.Microseconds()) / 1000.0,
		}
		// HW6: the search handler records how many products it checked and
		// matched, so the logs prove each search checks exactly 100.
		if v, ok := c.Get("products_checked"); ok {
			attrs = append(attrs, "products_checked", v)
		}
		if v, ok := c.Get("matches"); ok {
			attrs = append(attrs, "matches", v)
		}
		logger.Info("request", attrs...)
	}
}

// ---------- Product API ----------

// Product is exactly the schema from the yaml: what clients send and receive.
type Product struct {
	ProductID    int32  `json:"product_id"    binding:"required,min=1"`
	SKU          string `json:"sku"           binding:"required,min=1,max=100"`
	Manufacturer string `json:"manufacturer"  binding:"required,min=1,max=200"`
	CategoryID   int32  `json:"category_id"   binding:"required,min=1"`
	Weight       *int32 `json:"weight"        binding:"required,min=0"`
	SomeOtherID  int32  `json:"some_other_id" binding:"required,min=1"`
}

// storedProduct is the server's internal record: the public Product plus a
// Name that only the server uses for search (per instructor answer @829).
// Handlers always return sp.Product, so Name never reaches the client.
type storedProduct struct {
	Product
	Name string
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// SearchResponse matches the 200 response of GET /products/search.
type SearchResponse struct {
	Products   []Product `json:"products"`
	TotalFound int       `json:"total_found"`
	SearchTime string    `json:"search_time"`
}

// ---------- Data generation constants ----------

const (
	numProducts       = 100000 // memory footprint
	productsPerSearch = 100    // bounded iteration: fixed work per request
	maxResults        = 20     // cap on returned products
)

// Category names indexed by category_id - 1 (invented mapping).
var categories = []string{
	"Electronics", "Books", "Home", "Toys", "Sports",
	"Clothing", "Garden", "Beauty", "Automotive", "Grocery",
}

// Brand pool used in names: "Product <Brand> <ID>".
var brands = []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta", "Omega", "Nova"}

func categoryName(id int32) (string, bool) {
	idx := int(id) - 1
	if idx < 0 || idx >= len(categories) {
		return "", false
	}
	return categories[idx], true
}

// ---------- Store ----------

// ProductStore keeps products in a sync.Map (thread-safe, HW6 requirement)
// plus an ID-ordered slice so every search checks the same first 100 IDs.
// order is written once in Seed before the server starts and is read-only
// afterwards, so it needs no lock.
type ProductStore struct {
	products sync.Map // int32 -> storedProduct
	order    []int32
}

func NewProductStore() *ProductStore {
	return &ProductStore{}
}

// Seed generates n products with deterministic values.
func (s *ProductStore) Seed(n int) {
	s.order = make([]int32, 0, n)
	for i := 1; i <= n; i++ {
		id := int32(i)
		brand := brands[i%len(brands)]
		weight := int32((i * 37) % 5000)
		p := Product{
			ProductID:    id,
			SKU:          fmt.Sprintf("SKU-%06d", i),
			Manufacturer: brand,
			CategoryID:   int32(i%len(categories)) + 1, // rotate with modulo
			Weight:       &weight,
			SomeOtherID:  id,
		}
		s.products.Store(id, storedProduct{
			Product: p,
			Name:    fmt.Sprintf("Product %s %d", brand, i),
		})
		s.order = append(s.order, id)
	}
}

func (s *ProductStore) Get(id int32) (storedProduct, bool) {
	v, ok := s.products.Load(id)
	if !ok {
		return storedProduct{}, false
	}
	return v.(storedProduct), true
}

// Update replaces the public fields of an existing product and keeps its
// internal Name, since clients never send one. Returns false if id is unknown.
func (s *ProductStore) Update(p Product) bool {
	v, ok := s.products.Load(p.ProductID)
	if !ok {
		return false
	}
	old := v.(storedProduct)
	s.products.Store(p.ProductID, storedProduct{Product: p, Name: old.Name})
	return true
}

// ---------- Handlers ----------

func simulatedError(c *gin.Context) bool {
	if c.GetHeader("X-Simulate-Error") == "true" {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "INTERNAL_ERROR",
			Message: "Internal server error",
		})
		return true
	}
	return false
}

func getProduct(store *ProductStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if simulatedError(c) {
			return
		}

		id, err := strconv.ParseInt(c.Param("productId"), 10, 32)
		if err != nil || id < 1 {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Product not found",
				Details: "Product ID must be a positive 32-bit integer",
			})
			return
		}

		sp, ok := store.Get(int32(id))
		if !ok {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Product not found",
			})
			return
		}

		c.JSON(http.StatusOK, sp.Product) // schema only, no Name
	}
}

func addProductDetails(store *ProductStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if simulatedError(c) {
			return
		}

		id, err := strconv.ParseInt(c.Param("productId"), 10, 32)
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

		if !store.Update(p) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Product not found",
			})
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// searchProducts: GET /products/search?q=...
// Checks exactly productsPerSearch products (case-insensitive containment on
// name or category name) and returns at most maxResults of the matches.
func searchProducts(store *ProductStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if simulatedError(c) {
			return
		}

		q, present := c.GetQuery("q")
		qLen := utf8.RuneCountInString(q) // yaml lengths count characters, not bytes
		if !present || qLen < 1 || qLen > 100 {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "Invalid search query",
				Details: "query parameter q is required and must be 1-100 characters",
			})
			return
		}

		start := time.Now()
		needle := strings.ToLower(q)
		results := make([]Product, 0, maxResults)
		checked, matches := 0, 0

		for _, id := range store.order {
			if checked == productsPerSearch {
				break // bounded iteration: stop after N checked, NOT after N matched
			}
			sp, ok := store.Get(id)
			if !ok {
				continue
			}
			checked++

			cat, _ := categoryName(sp.CategoryID)
			if strings.Contains(strings.ToLower(sp.Name), needle) ||
				strings.Contains(strings.ToLower(cat), needle) {
				matches++
				if len(results) < maxResults {
					results = append(results, sp.Product)
				}
				// keep looping even after 20 results, so the work stays fixed
			}
		}
		elapsed := time.Since(start)

		c.Set("products_checked", checked)
		c.Set("matches", matches)

		c.JSON(http.StatusOK, SearchResponse{
			Products:   results,
			TotalFound: matches, // all matches among the checked products; products holds at most 20
			SearchTime: elapsed.String(),
		})
	}
}

func main() {
	store := NewProductStore()

	seedStart := time.Now()
	store.Seed(numProducts)
	logger.Info("seeded products", "count", numProducts, "took_ms", time.Since(seedStart).Milliseconds())

	// gin.New() instead of gin.Default(): drops Gin's text logger,
	// so each request produces only our one JSON log line.
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(observability())

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/products/search", searchProducts(store))
	r.GET("/products/:productId", getProduct(store))
	r.POST("/products/:productId/details", addProductDetails(store))

	r.Run(":8080")
}