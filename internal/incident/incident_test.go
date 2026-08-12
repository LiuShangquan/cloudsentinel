package incident

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestFingerprintAndEventKeyAreStable(t *testing.T) {
	starts := time.Date(2026, 8, 11, 1, 2, 3, 0, time.UTC)
	base := Alert{Labels: map[string]string{"alertname": "CloudSentinelProbeFailure", "service_id": "3", "task_id": "1", "probe_type": "http", "severity": "warning"}, Annotations: map[string]string{"summary": "first"}, StartsAt: starts, Fingerprint: "external-one"}
	first, err := normalize(base, starts)
	if err != nil {
		t.Fatal(err)
	}
	base.Annotations["summary"] = "changed"
	base.Fingerprint = "external-two"
	second, err := normalize(base, starts.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint != second.Fingerprint || first.EventKey != second.EventKey {
		t.Fatal("unstable fields changed identity")
	}
	base.StartsAt = starts.Add(time.Hour)
	third, _ := normalize(base, starts.Add(time.Hour))
	if third.Fingerprint != first.Fingerprint || third.EventKey == first.EventKey {
		t.Fatal("new occurrence identity is incorrect")
	}
}

func TestMachineAuthIsSeparateAndExact(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/webhook", MachineAuth("machine-token"), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, test := range []struct {
		name, authorization string
		want                int
	}{{"missing", "", 401}, {"wrong", "Bearer wrong", 401}, {"user jwt", "Bearer ey.user.jwt", 401}, {"correct", "Bearer machine-token", 204}} {
		request := httptest.NewRequest(http.MethodPost, "/webhook", nil)
		if test.authorization != "" {
			request.Header.Set("Authorization", test.authorization)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Errorf("%s status=%d want=%d", test.name, response.Code, test.want)
		}
	}
}
