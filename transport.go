package transport

import (
	"bytes"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

// Transport is an http roundtripper.
type Transport struct {
	m         *metrics.Set
	rt        http.RoundTripper
	name      string
	mr        map[*regexp.Regexp]string
	normalize bool
}

// Prefs for transport.
type Prefs struct {
	// Metrics set.
	Metrics *metrics.Set

	// Flag to normalize http requests.
	Normalize bool

	// Name of the service.
	Name string

	// Transport http requests.
	Transport http.RoundTripper

	// MatchRoute will match against regex and assigns corresponding label.
	MatchRoute map[*regexp.Regexp]string
}

// Metrics is a global interface type for metrics exporters to export data.
type Metrics interface {
	Export() []byte
}

// NewTransport is a Transport object generator.
func NewTransport(p *Prefs) *Transport {
	if p.Transport == nil {
		p.Transport = http.DefaultTransport
	}

	if p.Metrics == nil {
		p.Metrics = metrics.NewSet()
	}

	if p.Name == "" {
		p.Name = "client-metrics"
	}

	return &Transport{
		m:         p.Metrics,
		name:      p.Name,
		rt:        p.Transport,
		mr:        p.MatchRoute,
		normalize: p.Normalize,
	}
}

// RoundTrip to make http request.
func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	var (
		start      = time.Now()
		statusCode string
	)

	resp, err := t.rt.RoundTrip(r)

	if err != nil {
		return resp, err
	}

	statusCode = strconv.Itoa(resp.StatusCode)

	if t.normalize {
		statusCode = string(statusCode[0]) + "XX"
	}

	u := resp.Request.URL.String()

	var flag = false

	for k, v := range t.mr {
		if k.MatchString(u) {
			u = v
			flag = true
		}

		if flag {
			break
		}
	}

	t.m.GetOrCreateHistogram(
		fmt.Sprintf(
			`http_client_request_duration_seconds{status="%s",method="%s",url="%s",module_name="%s"}`,
			statusCode, resp.Request.Method, u, t.name,
		),
	).UpdateDuration(start)

	return resp, err
}

// Export metrics data as byte array.
func (t *Transport) Export() []byte {
	var buf = new(bytes.Buffer)

	// Write data in metrics to the buffer.
	t.m.WritePrometheus(buf)

	// Return buffer in bytes.
	return buf.Bytes()
}

// HandleRequest to http handler.
func (t *Transport) HandleRequest(w http.ResponseWriter, r *http.Request) {
	t.m.WritePrometheus(w)
}
