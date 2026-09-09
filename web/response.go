package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ISeekFree/AtlasGo/common"
	"github.com/gin-gonic/gin"
)

func (s *SDK) ResponseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled(s.options.Enabled) || !enabled(s.options.Response.Wrap) {
			c.Next()
			return
		}
		recorder := &responseRecorder{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = recorder
		c.Next()

		body := recorder.body.Bytes()
		status := recorder.Status()
		if !shouldWrapResponse(c, status, body, s.options.Response.NotWrapPrefixes) {
			recorder.flushOriginal(status, body)
			return
		}

		data := decodeBodyData(c.Writer.Header().Get("Content-Type"), body)
		c.Writer = recorder.ResponseWriter
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Length", "")
		c.Status(status)
		_ = json.NewEncoder(c.Writer).Encode(common.Success(data))
	}
}

type responseRecorder struct {
	gin.ResponseWriter
	body    bytes.Buffer
	status  int
	size    int
	written bool
}

func (r *responseRecorder) WriteHeader(code int) {
	if code > 0 {
		r.status = code
	}
	r.written = true
}

func (r *responseRecorder) WriteHeaderNow() {
	r.written = true
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	r.written = true
	r.size += len(data)
	return r.body.Write(data)
}

func (r *responseRecorder) WriteString(data string) (int, error) {
	r.written = true
	r.size += len(data)
	return r.body.WriteString(data)
}

func (r *responseRecorder) Status() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}

func (r *responseRecorder) Size() int {
	return r.size
}

func (r *responseRecorder) Written() bool {
	return r.written
}

func (r *responseRecorder) flushOriginal(status int, body []byte) {
	original := r.ResponseWriter
	original.WriteHeader(status)
	if len(body) > 0 {
		_, _ = original.Write(body)
	}
}

func shouldWrapResponse(c *gin.Context, status int, body []byte, prefixes []string) bool {
	if len(body) == 0 || status < http.StatusOK || status >= http.StatusMultipleChoices {
		return false
	}
	for _, prefix := range prefixes {
		if prefix != "" && strings.HasPrefix(c.Request.URL.Path, prefix) {
			return false
		}
	}
	contentType := c.Writer.Header().Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") || strings.Contains(contentType, "octet-stream") {
		return false
	}
	return !alreadyResponse(body)
}

func alreadyResponse(body []byte) bool {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return false
	}
	_, hasCode := probe["code"]
	_, hasMsg := probe["msg"]
	_, hasData := probe["data"]
	return hasCode && (hasMsg || hasData)
}

func decodeBodyData(contentType string, body []byte) any {
	if strings.Contains(contentType, "application/json") {
		var value any
		if err := json.Unmarshal(body, &value); err == nil {
			return value
		}
	}
	return string(body)
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, common.Success(data))
}

func Fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, common.Failure(code, msg))
}
