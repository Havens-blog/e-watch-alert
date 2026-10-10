package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status 单一降级因子的状态。
type Status string

const (
	StatusUp       Status = "up"
	StatusDegraded Status = "degraded"
)

// AuthHealth 探针对外暴露的聚合状态（GET /api/watchalert/auth-health 的 data）。
type AuthHealth struct {
	ProbeA        Status `json:"probe_a"`
	ProbeB        Status `json:"probe_b"`
	WriteDisabled bool   `json:"write_disabled"`
}

// state 单一降级因子的滞回状态机（连续 hysteresis 次同向才翻转，1-2 次不触发）。
type state struct {
	mu         sync.RWMutex
	status     Status
	hysteresis int
	sameCount  int
}

func newState(hysteresis int) *state {
	if hysteresis < 1 {
		hysteresis = 3
	}
	return &state{status: StatusUp, hysteresis: hysteresis}
}

// observe 记录一次探测结果；达到连续阈值才翻转状态。
func (s *state) observe(ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	target := StatusDegraded
	if ok {
		target = StatusUp
	}
	if s.status == target {
		s.sameCount = 0
		return
	}
	s.sameCount++
	if s.sameCount >= s.hysteresis {
		s.status = target
		s.sameCount = 0
	}
}

func (s *state) get() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// ProbeA 主动探 eiam GET /health（10s 间隔、3s 超时；判定 2xx 且信封 code===0）。
type ProbeA struct {
	baseURL  string
	client   *http.Client
	interval time.Duration
	timeout  time.Duration
	state    *state
}

func NewProbeA(baseURL string, interval, timeout time.Duration, hysteresis int) *ProbeA {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &ProbeA{
		baseURL:  baseURL,
		client:   &http.Client{Timeout: timeout},
		interval: interval,
		timeout:  timeout,
		state:    newState(hysteresis),
	}
}

// Enabled 探针 A 是否启用（baseURL 空=禁用，始终 healthy）。
func (p *ProbeA) Enabled() bool { return p.baseURL != "" }

// Run 后台循环探测（阻塞直至 ctx 取消）。
func (p *ProbeA) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.state.observe(p.probeOnce())
		}
	}
}

// probeOnce 单次探测：2xx 且信封 code===0 才成功；传输/解析失败均计失败。
func (p *ProbeA) probeOnce() bool {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL, nil)
	if err != nil {
		return false
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}
	var env struct {
		Code int `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return false
	}
	return env.Code == 0
}

func (p *ProbeA) Degraded() bool { return p.state.get() == StatusDegraded }

// ProbeB Redis PING 滞回状态（每次 Auth() 读会话附带记录，与探针 A 解耦）。
type ProbeB struct {
	state *state
}

func NewProbeB(hysteresis int) *ProbeB { return &ProbeB{state: newState(hysteresis)} }

func (p *ProbeB) Record(ok bool) { p.state.observe(ok) }
func (p *ProbeB) Degraded() bool { return p.state.get() == StatusDegraded }

// Health 聚合降级状态。
type Health struct {
	A *ProbeA
	B *ProbeB
}

func NewHealth(a *ProbeA, b *ProbeB) *Health { return &Health{A: a, B: b} }

func (h *Health) WriteDisabled() bool {
	return h.A.Degraded() || h.B.Degraded()
}

func (h *Health) Snapshot() AuthHealth {
	a, b := StatusUp, StatusUp
	if h.A.Degraded() {
		a = StatusDegraded
	}
	if h.B.Degraded() {
		b = StatusDegraded
	}
	return AuthHealth{ProbeA: a, ProbeB: b, WriteDisabled: h.WriteDisabled()}
}

// ==================== 全局默认实例（进程级单例，供 middleware/auth/api 使用） ====================

var defaultHealth = NewHealth(NewProbeA("", 0, 0, 3), NewProbeB(3))

// Init 用部署配置初始化全局探针（main 启动时调用）。
func Init(eiamHealthURL string, intervalSec, timeoutSec, hysteresis int) {
	defaultHealth.A = NewProbeA(eiamHealthURL, time.Duration(intervalSec)*time.Second, time.Duration(timeoutSec)*time.Second, hysteresis)
	defaultHealth.B = NewProbeB(hysteresis)
}

func Default() *Health { return defaultHealth }

// Start 启动探针 A 后台循环（ctx 取消即退出；未启用则立即返回）。
func Start(ctx context.Context) {
	if defaultHealth.A.Enabled() {
		go defaultHealth.A.Run(ctx)
	}
}

func WriteDisabled() bool  { return defaultHealth.WriteDisabled() }
func Snapshot() AuthHealth { return defaultHealth.Snapshot() }
func RecordProbeB(ok bool) { defaultHealth.B.Record(ok) }

// ResetForTests 重置全局状态（测试隔离用）。
func ResetForTests() {
	defaultHealth = NewHealth(NewProbeA("", 0, 0, 3), NewProbeB(3))
}
