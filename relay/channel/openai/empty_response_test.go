package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newEmptyResponseTestContext(path string, body string, contentType string) (*gin.Context, *httptest.ResponseRecorder, *http.Response) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{contentType}},
	}
	return c, recorder, resp
}

func TestOpenaiHandlerReturnsRetryableErrorForEmptyResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, body := range []string{"", " \n\t", `{}`, `{"choices":[]}`} {
		t.Run(body, func(t *testing.T) {
			c, recorder, resp := newEmptyResponseTestContext("/v1/chat/completions", body, "application/json")
			_, apiErr := OpenaiHandler(c, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}, resp)

			require.Nil(t, apiErr)
			assert.Equal(t, strings.TrimSpace(body), recorder.Body.String())
		})
	}
}

func TestOaiResponsesHandlerReturnsRetryableErrorForEmptyResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, body := range []string{"", " \n\t", `{}`, `{"output":[]}`} {
		t.Run(body, func(t *testing.T) {
			c, recorder, resp := newEmptyResponseTestContext("/v1/responses", body, "application/json")
			_, apiErr := OaiResponsesHandler(c, &relaycommon.RelayInfo{}, resp)

			require.Nil(t, apiErr)
			assert.Equal(t, strings.TrimSpace(body), recorder.Body.String())
		})
	}
}

func TestOpenAIStreamHandlersReturnRetryableErrorWithoutDataEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	tests := []struct {
		name    string
		path    string
		body    string
		handler func(*gin.Context, *relaycommon.RelayInfo, *http.Response) *types.NewAPIError
	}{
		{
			name: "chat empty body",
			path: "/v1/chat/completions",
			handler: func(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) *types.NewAPIError {
				_, apiErr := OaiStreamHandler(c, info, resp)
				return apiErr
			},
		},
		{
			name: "chat done only",
			path: "/v1/chat/completions",
			body: "data: [DONE]\n\n",
			handler: func(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) *types.NewAPIError {
				_, apiErr := OaiStreamHandler(c, info, resp)
				return apiErr
			},
		},
		{
			name: "responses empty body",
			path: "/v1/responses",
			handler: func(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) *types.NewAPIError {
				_, apiErr := OaiResponsesStreamHandler(c, info, resp)
				return apiErr
			},
		},
		{
			name: "responses done only",
			path: "/v1/responses",
			body: "data: [DONE]\n\n",
			handler: func(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) *types.NewAPIError {
				_, apiErr := OaiResponsesStreamHandler(c, info, resp)
				return apiErr
			},
		},
		{
			name: "responses lifecycle events only",
			path: "/v1/responses",
			body: "data: {\"type\":\"response.created\",\"response\":{\"status\":\"in_progress\"}}\n\n" +
				"data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"reasoning\",\"status\":\"in_progress\"}}\n\n" +
				"data: {\"type\":\"response.content_part.added\",\"part\":{\"type\":\"output_text\",\"text\":\"\"}}\n\n",
			handler: func(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) *types.NewAPIError {
				_, apiErr := OaiResponsesStreamHandler(c, info, resp)
				return apiErr
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, _, resp := newEmptyResponseTestContext(test.path, test.body, "text/event-stream")
			info := &relaycommon.RelayInfo{
				RelayMode:   relayconstant.RelayModeChatCompletions,
				RelayFormat: types.RelayFormatOpenAI,
				ChannelMeta: &relaycommon.ChannelMeta{},
				DisablePing: true,
			}

			apiErr := test.handler(c, info, resp)

			require.Nil(t, apiErr)
		})
	}
}

func TestOaiResponsesStreamHandlerReturnsRetryableErrorForServerOverload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	for _, code := range []string{"server_is_overloaded", "slow_down"} {
		t.Run(code, func(t *testing.T) {
			body := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"status\":\"in_progress\"}}\n\n" +
				"data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"message\":\"Selected model is at capacity. Please try a different model.\",\"type\":\"service_unavailable_error\",\"code\":\"" + code + "\"}}}\n\n"
			c, recorder, resp := newEmptyResponseTestContext("/v1/responses", body, "text/event-stream")
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{},
				DisablePing: true,
			}

			_, apiErr := OaiResponsesStreamHandler(c, info, resp)

			require.NotNil(t, apiErr)
			assert.Equal(t, types.ErrorCode(code), apiErr.GetErrorCode())
			assert.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
			assert.Empty(t, recorder.Body.String())
		})
	}
}

func TestOaiResponsesStreamHandlerDoesNotRetryServerOverloadAfterOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n" +
		"data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"message\":\"Selected model is at capacity. Please try a different model.\",\"type\":\"service_unavailable_error\",\"code\":\"server_is_overloaded\"}}}\n\n"
	c, recorder, resp := newEmptyResponseTestContext("/v1/responses", body, "text/event-stream")
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
		DisablePing: true,
	}

	_, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	assert.Contains(t, recorder.Body.String(), `"delta":"partial"`)
	assert.Contains(t, recorder.Body.String(), `"code":"server_is_overloaded"`)
}
