package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/deaprima/linkforge/services/gateway/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// rpcContext meneruskan IP & User-Agent client ke service internal via gRPC metadata.
// "user-agent" adalah header reserved di grpc-go, jadi dikirim sebagai "x-user-agent".
func rpcContext(c *gin.Context) context.Context {
	return metadata.AppendToOutgoingContext(c.Request.Context(),
		"x-forwarded-for", c.ClientIP(),
		"x-user-agent", c.Request.UserAgent(),
	)
}

// bindJSON mem-parse & memvalidasi body. Kalau gagal, response 400 sudah ditulis dan return false.
// allowEmpty dipakai endpoint yang body-nya opsional (mis. refresh/logout berbasis cookie).
func bindJSON(c *gin.Context, dst any, allowEmpty bool) bool {
	err := c.ShouldBindJSON(dst)
	if err == nil || (allowEmpty && errors.Is(err, io.EOF)) {
		return true
	}
	response.Error(c, http.StatusBadRequest, response.CodeValidation, validationMessage(err))
	return false
}

// validationMessage mengubah error validator menjadi pesan yang bisa dibaca client.
// Nama field mengikuti tag json (lihat RegisterJSONTagNames di package router).
func validationMessage(err error) string {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		if errors.Is(err, io.EOF) {
			return "Request body is required."
		}
		return "Request body is not valid JSON."
	}

	msgs := make([]string, 0, len(verrs))
	for _, fe := range verrs {
		field := fe.Field()
		switch fe.Tag() {
		case "required":
			msgs = append(msgs, field+" is required")
		case "email":
			msgs = append(msgs, field+" must be a valid email address")
		case "url", "http_url":
			msgs = append(msgs, field+" must be a valid http(s) URL")
		case "min":
			msgs = append(msgs, fmt.Sprintf("%s must be at least %s characters", field, fe.Param()))
		case "max":
			msgs = append(msgs, fmt.Sprintf("%s must be at most %s characters", field, fe.Param()))
		default:
			msgs = append(msgs, field+" is invalid")
		}
	}
	return strings.Join(msgs, "; ")
}

func timePtr(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}

func timeVal(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

func toTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
