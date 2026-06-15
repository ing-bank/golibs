package tripperware

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/ing-bank/golibs/pkg/http/response"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
)

var promLabels = []string{"method", "host"} // "path"
// Metrics variables are at end of file

func Metrics(logTransportTimesOnErr bool) Tripperware {
	return func(next Endpoint) Endpoint {
		return func(ctx context.Context, request *http.Request) *response.Data {
			labels := prometheus.Labels{
				"method": request.Method,
				"host":   request.Host,
				//"path":   fuzzURL(request.URL.String()),
			}

			trace, finish := newTrace(ctx, labels, logTransportTimesOnErr)
			promRequestSizeBytes.With(labels).Observe(float64(request.ContentLength))
			request = request.WithContext(httptrace.WithClientTrace(ctx, trace))

			resp := next(ctx, request)

			//labels["status"] = strconv.Itoa(resp.Status)
			finish(resp)

			return resp
		}
	}
}

func fuzzURL(url string) string {
	// uuidRegex := "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}}"
	return url
}

func newTrace(ctx context.Context, labels prometheus.Labels, allowLog bool) (*httptrace.ClientTrace, func(response *response.Data)) {
	var start, dnsStart, connectStart, tlsStart time.Time
	var end, dnsEnd, connectEnd, tlsEnd, firstResponse time.Duration
	start = time.Now()

	afterReq := func(response *response.Data) {
		end = time.Since(start)

		// Log failed or slow requests
		if allowLog && (response.Status == 0 || end.Seconds() > 0.5) {
			log.WithContext(ctx).Infof("[HttpClient] Metrics: dns: %.2fs, connect: %.2fs, tls: %.2fs, firstResponse: %.2fs, full: %.2fs",
				dnsEnd.Seconds(), connectEnd.Seconds(), tlsEnd.Seconds(), firstResponse.Seconds(), end.Seconds(),
			)
		}

		promRequestDurationSec.With(labels).Observe(end.Seconds())
		promRequestCount.With(labels).Inc()
		promResponseSizeBytes.With(labels).Observe(float64(len(response.Raw)))
	}

	return &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			if !dnsStart.IsZero() {
				dnsEnd = time.Since(dnsStart)
				dnsDurationSec.With(labels).Observe(dnsEnd.Seconds())
			}
		},
		ConnectStart: func(network, addr string) {
			connectStart = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			if !connectStart.IsZero() {
				connectEnd = time.Since(connectStart)
				connectDurationSec.With(labels).Observe(connectEnd.Seconds())
			}
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			if !tlsStart.IsZero() {
				tlsEnd = time.Since(tlsStart)
				tlsDurationSec.With(labels).Observe(tlsEnd.Seconds())
			}
		},
		GotFirstResponseByte: func() {
			firstResponse = time.Since(start)
			firstResponseDurationSec.With(labels).Observe(firstResponse.Seconds())
		},
	}, afterReq
}

// --- Metrics variables below ---

var promRequestCount = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "golibs_http_client_request_count_total",
		Help: "Total number of HTTP requests made.",
	}, promLabels,
)

var promRequestDurationSec = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "golibs_http_client_request_duration_seconds",
		Help:    "HTTP request latencies in seconds.",
		Buckets: prometheus.DefBuckets,
	}, promLabels,
)

var promRequestSizeBytes = promauto.NewSummaryVec(
	prometheus.SummaryOpts{
		Name: "golibs_http_client_request_size_bytes",
		Help: "HTTP request sizes in bytes.",
	}, promLabels,
)

var promResponseSizeBytes = promauto.NewSummaryVec(
	prometheus.SummaryOpts{
		Name: "golibs_http_client_response_size_bytes",
		Help: "HTTP response sizes in bytes.",
	}, promLabels,
)

var dnsDurationSec = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "golibs_http_client_dns_duration_seconds",
		Help:    "HTTP DNS request latencies in seconds.",
		Buckets: prometheus.DefBuckets,
	}, promLabels,
)

var connectDurationSec = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "golibs_http_client_connection_duration_seconds",
		Help:    "HTTP connect request latencies in seconds.",
		Buckets: prometheus.DefBuckets,
	}, promLabels,
)

var tlsDurationSec = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "golibs_http_client_tls_duration_seconds",
		Help:    "HTTP tls request latencies in seconds.",
		Buckets: prometheus.DefBuckets,
	}, promLabels,
)

var firstResponseDurationSec = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "golibs_http_client_first_response_duration_seconds",
		Help:    "HTTP request first response byte latencies in seconds.",
		Buckets: prometheus.DefBuckets,
	}, promLabels,
)
