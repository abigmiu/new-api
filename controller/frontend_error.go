package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/gin-gonic/gin"
)

type frontendErrorReport struct {
	Type      string `json:"type"`
	Message   string `json:"message"`
	Stack     string `json:"stack"`
	URL       string `json:"url"`
	UserAgent string `json:"user_agent"`
	Source    string `json:"source"`
}

// ReportFrontendError writes sanitized frontend errors into the active backend log.
func ReportFrontendError(c *gin.Context) {
	var req frontendErrorReport
	if err := common.DecodeJson(http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024), &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}

	errorType := cleanFrontendLogField(req.Type, 40)
	if errorType == "" {
		errorType = "unknown"
	}
	message := cleanFrontendLogField(req.Message, 500)
	if message == "" {
		message = "empty message"
	}
	stack := cleanFrontendLogField(req.Stack, 2000)
	url := cleanFrontendLogField(req.URL, 300)
	userAgent := cleanFrontendLogField(req.UserAgent, 200)
	source := cleanFrontendLogField(req.Source, 80)

	logger.LogError(c.Request.Context(), fmt.Sprintf(
		"[FRONTEND_ERROR] type=%q source=%q url=%q message=%q stack=%q user_agent=%q client_ip=%q",
		errorType,
		source,
		url,
		message,
		stack,
		userAgent,
		c.ClientIP(),
	))

	common.ApiSuccess(c, nil)
}

func cleanFrontendLogField(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\t", " ")
	if len(value) > maxLen {
		return value[:maxLen]
	}
	return value
}
