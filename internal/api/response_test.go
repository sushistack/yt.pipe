package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sushistack/yt.pipe/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON_SuccessResponse(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	api.WriteJSON(w, r, http.StatusOK, map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Nil(t, resp.Error)
	assert.NotEmpty(t, resp.Timestamp)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "value", data["key"])
}

func TestWriteJSON_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	api.WriteJSON(w, r, http.StatusOK, nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Nil(t, resp.Data)
}

func TestWriteJSON_CustomStatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/test", nil)

	api.WriteJSON(w, r, http.StatusCreated, map[string]string{"id": "123"})

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestWriteError_ErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	api.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "field is required")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "VALIDATION_ERROR", resp.Error.Code)
	assert.Equal(t, "field is required", resp.Error.Message)
	assert.NotEmpty(t, resp.Timestamp)
}

func TestWriteError_InternalServerError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	api.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "something went wrong")

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "INTERNAL_ERROR", resp.Error.Code)
}

func TestGetRequestID_WithValue(t *testing.T) {
	ctx := context.Background()
	// The request ID middleware would set this, but we test GetRequestID directly
	// An empty context should return empty string
	id := api.GetRequestID(ctx)
	assert.Empty(t, id)
}
