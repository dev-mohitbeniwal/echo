// api/controller/policy_controller_test.go
package controller_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dev-mohitbeniwal/echo/api/controller"
	echo_errors "github.com/dev-mohitbeniwal/echo/api/errors"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dev-mohitbeniwal/echo/api/model"
	mock_service "github.com/dev-mohitbeniwal/echo/api/test/service_mock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func setupRouter() *gin.Engine {
	r := gin.Default()
	return r
}

func TestPolicyController(t *testing.T) {
	// Initialize logger
	logger.InitLogger("../logging")
	defer logger.Sync()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPolicyService := mock_service.NewMockIPolicyService(ctrl)
	policyController := controller.NewPolicyController(mockPolicyService)
	router := setupRouter()
	api := router.Group("/")
	policyController.RegisterRoutes(api)

	t.Run("CreatePolicy_Success", func(t *testing.T) {
		mockPolicyService.EXPECT().
			CreatePolicy(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&model.Policy{ID: "1", Name: "Test Policy"}, nil)

		body := strings.NewReader(`{"name":"Test Policy"}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/policies", body)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("UpdatePolicy_Success", func(t *testing.T) {
		mockPolicyService.EXPECT().
			UpdatePolicy(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&model.Policy{ID: "1", Name: "Updated Policy"}, nil)

		body := strings.NewReader(`{"name":"Updated Policy"}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/policies/1", body)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("UpdatePolicy_Failure_NotFound", func(t *testing.T) {
		mockPolicyService.EXPECT().
			UpdatePolicy(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, echo_errors.ErrPolicyNotFound)

		body := strings.NewReader(`{"name":"Updated Policy"}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/policies/1", body)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("DeletePolicy_Success", func(t *testing.T) {
		mockPolicyService.EXPECT().
			DeletePolicy(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/policies/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("DeletePolicy_Failure_NotFound", func(t *testing.T) {
		mockPolicyService.EXPECT().
			DeletePolicy(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(echo_errors.ErrPolicyNotFound)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/policies/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("GetPolicy_Success", func(t *testing.T) {
		mockPolicyService.EXPECT().
			GetPolicy(gomock.Any(), gomock.Any()).
			Return(&model.Policy{ID: "1", Name: "Test Policy"}, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/policies/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GetPolicy_Failure_NotFound", func(t *testing.T) {
		mockPolicyService.EXPECT().
			GetPolicy(gomock.Any(), gomock.Any()).
			Return(nil, echo_errors.ErrPolicyNotFound)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/policies/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("ListPolicies_Success", func(t *testing.T) {
		policies := []*model.Policy{
			{ID: "1", Name: "Policy 1"},
			{ID: "2", Name: "Policy 2"},
		}

		mockPolicyService.EXPECT().
			ListPolicies(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(policies, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/policies", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SearchPolicies_Success", func(t *testing.T) {
		policies := []*model.Policy{
			{ID: "1", Name: "Policy 1"},
			{ID: "2", Name: "Policy 2"},
		}

		mockPolicyService.EXPECT().
			SearchPolicies(gomock.Any(), gomock.Any()).
			Return(policies, nil)

		body := strings.NewReader(`{"name":"Policy"}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/policies/search", body)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("AnalyzePolicyUsage_Success", func(t *testing.T) {
		// Use a fixed time for testing.
		fixedTime := time.Date(2024, time.July, 7, 19, 55, 23, 999575000, time.Local)
		analysis := &model.PolicyUsageAnalysis{
			PolicyID:       "1",
			PolicyName:     "Test Policy",
			ResourceCount:  5,
			SubjectCount:   10,
			ConditionCount: 3,
			CreatedAt:      fixedTime,
			LastUpdatedAt:  fixedTime,
		}

		// Mock the service to return the predefined analysis object.
		mockPolicyService.EXPECT().
			AnalyzePolicyUsage(gomock.Any(), gomock.Any()).
			Return(analysis, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/policies/1/usage", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var responseAnalysis model.PolicyUsageAnalysis
		json.NewDecoder(w.Body).Decode(&responseAnalysis)

		// Assert individual fields to avoid issues with time comparison.
		assert.Equal(t, analysis.PolicyID, responseAnalysis.PolicyID)
		assert.Equal(t, analysis.PolicyName, responseAnalysis.PolicyName)
		assert.Equal(t, analysis.ResourceCount, responseAnalysis.ResourceCount)
		assert.Equal(t, analysis.SubjectCount, responseAnalysis.SubjectCount)
		assert.Equal(t, analysis.ConditionCount, responseAnalysis.ConditionCount)
		// Compare times using Equal to ignore internal representation differences.
		assert.True(t, analysis.CreatedAt.Equal(responseAnalysis.CreatedAt))
		assert.True(t, analysis.LastUpdatedAt.Equal(responseAnalysis.LastUpdatedAt))
	})

}
