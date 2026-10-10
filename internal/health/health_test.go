package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestState_Hysteresis(t *testing.T) {
	s := newState(3)
	assert.Equal(t, StatusUp, s.get())

	// 1-2 次失败不触发 degraded
	s.observe(false)
	assert.Equal(t, StatusUp, s.get())
	s.observe(false)
	assert.Equal(t, StatusUp, s.get())

	// 第 3 次失败 → degraded
	s.observe(false)
	assert.Equal(t, StatusDegraded, s.get())

	// 1-2 次成功不恢复
	s.observe(true)
	assert.Equal(t, StatusDegraded, s.get())
	s.observe(true)
	assert.Equal(t, StatusDegraded, s.get())

	// 第 3 次成功 → recovered
	s.observe(true)
	assert.Equal(t, StatusUp, s.get())
}

func TestState_ConsecutiveReset(t *testing.T) {
	s := newState(3)
	// fail → success(重置连续计数) → fail → fail，只有连续 2 次失败，不触发
	s.observe(false)
	s.observe(true)
	s.observe(false)
	s.observe(false)
	assert.Equal(t, StatusUp, s.get())
}

func TestProbeA_ProbeOnce_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok","data":{}}`))
	}))
	defer ts.Close()

	p := NewProbeA(ts.URL+"/health", time.Second, time.Second, 3)
	assert.True(t, p.probeOnce())
}

func TestProbeA_ProbeOnce_NonZeroCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"degraded","data":{}}`))
	}))
	defer ts.Close()

	p := NewProbeA(ts.URL+"/health", time.Second, time.Second, 3)
	assert.False(t, p.probeOnce())
}

func TestProbeA_ProbeOnce_HTTP500(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer ts.Close()

	p := NewProbeA(ts.URL+"/health", time.Second, time.Second, 3)
	assert.False(t, p.probeOnce())
}

func TestProbeA_ProbeOnce_TransportFailure(t *testing.T) {
	// 传输层故障（连接被拒）不应误判为成功，也不 panic
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := ts.URL + "/health"
	ts.Close()

	p := NewProbeA(url, time.Second, 500*time.Millisecond, 3)
	assert.False(t, p.probeOnce())
}

func TestProbeA_Disabled(t *testing.T) {
	p := NewProbeA("", time.Second, time.Second, 3)
	assert.False(t, p.Enabled())
	assert.False(t, p.Degraded())
}

func TestHealth_WriteDisabled(t *testing.T) {
	a := NewProbeA("", time.Second, time.Second, 3) // 禁用
	b := NewProbeB(3)
	h := NewHealth(a, b)
	assert.False(t, h.WriteDisabled())

	// 探针 B 连续 3 失败 → degraded → write_disabled
	b.Record(false)
	b.Record(false)
	assert.False(t, h.WriteDisabled(), "1-2 次失败不触发")

	b.Record(false)
	assert.True(t, b.Degraded())
	assert.True(t, h.WriteDisabled())

	snap := h.Snapshot()
	assert.True(t, snap.WriteDisabled)
	assert.Equal(t, StatusDegraded, snap.ProbeB)
	assert.Equal(t, StatusUp, snap.ProbeA)
}
