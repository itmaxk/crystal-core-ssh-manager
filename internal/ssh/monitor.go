package ssh

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"
)

type HostTarget struct {
	HostID   string
	Hostname string
	Port     int
}

type PingResult struct {
	HostID    string    `json:"host_id"`
	Online    bool      `json:"online"`
	LatencyMs *int64    `json:"latency_ms"`
	CheckedAt time.Time `json:"checked_at" ts_type:"string"`
	Error     string    `json:"error,omitempty"`
}

type MonitorConfig struct {
	Interval     time.Duration
	DialTimeout  time.Duration
	MaxParallel  int
	FetchTargets func(ctx context.Context) ([]HostTarget, error)
	OnUpdate     func(result PingResult)
}

type ReachabilityMonitor struct {
	cfg       MonitorConfig
	triggerCh chan struct{}
	mu        sync.RWMutex
	results   map[string]PingResult
}

func NewReachabilityMonitor(cfg MonitorConfig) *ReachabilityMonitor {
	if cfg.Interval <= 0 {
		cfg.Interval = 45 * time.Second
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 2 * time.Second
	}
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = 10
	}
	return &ReachabilityMonitor{
		cfg:       cfg,
		triggerCh: make(chan struct{}, 1),
		results:   make(map[string]PingResult),
	}
}

func (m *ReachabilityMonitor) Start(ctx context.Context) {
	m.checkAll(ctx)
	ticker := time.NewTicker(m.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAll(ctx)
		case <-m.triggerCh:
			m.checkAll(ctx)
			ticker.Reset(m.cfg.Interval)
		}
	}
}

func (m *ReachabilityMonitor) RefreshNow() {
	select {
	case m.triggerCh <- struct{}{}:
	default:
	}
}

func (m *ReachabilityMonitor) CheckHost(ctx context.Context, target HostTarget) PingResult {
	res := m.dialTarget(ctx, target)
	m.saveAndEmit(res)
	return res
}

func (m *ReachabilityMonitor) Snapshot() map[string]PingResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]PingResult, len(m.results))
	for k, v := range m.results {
		out[k] = v
	}
	return out
}

func (m *ReachabilityMonitor) checkAll(ctx context.Context) {
	if m.cfg.FetchTargets == nil {
		return
	}
	targets, err := m.cfg.FetchTargets(ctx)
	if err != nil || len(targets) == 0 {
		return
	}

	sem := make(chan struct{}, m.cfg.MaxParallel)
	var wg sync.WaitGroup

	for _, t := range targets {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(target HostTarget) {
			defer wg.Done()
			defer func() { <-sem }()
			res := m.dialTarget(ctx, target)
			m.saveAndEmit(res)
		}(t)
	}
	wg.Wait()
}

func (m *ReachabilityMonitor) dialTarget(ctx context.Context, target HostTarget) PingResult {
	addr := net.JoinHostPort(target.Hostname, strconv.Itoa(target.Port))
	dialer := net.Dialer{Timeout: m.cfg.DialTimeout}

	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	elapsedMs := time.Since(start).Milliseconds()

	res := PingResult{
		HostID:    target.HostID,
		CheckedAt: time.Now(),
	}
	if err != nil {
		res.Online = false
		res.Error = err.Error()
		return res
	}

	_ = conn.Close()
	res.Online = true
	res.LatencyMs = &elapsedMs
	return res
}

func (m *ReachabilityMonitor) saveAndEmit(res PingResult) {
	m.mu.Lock()
	m.results[res.HostID] = res
	m.mu.Unlock()
	if m.cfg.OnUpdate != nil {
		m.cfg.OnUpdate(res)
	}
}
