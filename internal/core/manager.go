package core

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Snapshot struct {
	Phase    string    `json:"phase"`
	Error    string    `json:"error"`
	Port     int       `json:"port"`
	Data     string    `json:"data"`
	Logs     []LogLine `json:"logs"`
	Settings Settings  `json:"settings"`
}
type Manager struct {
	mu                       sync.Mutex
	paths                    Paths
	settings                 Settings
	log                      Log
	phase, lastError, launch string
	lanIP                    string
	port                     int
	cancel                   context.CancelFunc
	done                     chan struct{}
}

func NewManager(p Paths, s Settings) *Manager {
	return &Manager{paths: p, settings: s, phase: "stopped", log: Log{path: filepath.Join(p.Logs, "runtime.log")}}
}
func (m *Manager) ReportError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.phase = "error"
	m.lastError = Redact(err.Error())
	m.log.Add(m.lastError)
}
func (m *Manager) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Snapshot{m.phase, m.lastError, m.port, m.paths.Data, m.log.Lines(), m.settings}
}
func (m *Manager) Configure(s Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return errors.New("请先停止服务再修改设置")
	}
	if err := m.paths.SaveSettings(s); err != nil {
		return err
	}
	m.settings = s
	return nil
}

// The bearer launch URL is deliberately excluded from every status snapshot.
// Only an explicit open/copy/share request can retrieve this in-memory secret.
func (m *Manager) LaunchURL() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.phase != "running" {
		return "", errors.New("服务尚未就绪")
	}
	return m.launch, nil
}
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.done = make(chan struct{})
	m.phase = "installing"
	m.lastError = ""
	m.launch = ""
	s := m.settings
	go m.run(ctx, s, m.done)
	return nil
}
func (m *Manager) Stop() {
	m.mu.Lock()
	cancel, done := m.cancel, m.done
	m.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
func (m *Manager) run(ctx context.Context, s Settings, done chan struct{}) {
	var failure error
	defer func() {
		m.mu.Lock()
		m.cancel = nil
		m.launch = ""
		m.lanIP = ""
		m.phase = "stopped"
		if failure != nil && ctx.Err() == nil {
			m.phase = "error"
			m.lastError = Redact(failure.Error())
			m.log.Add(m.lastError)
		}
		close(done)
		m.mu.Unlock()
	}()
	installer := Installer{m.paths, s, &m.log}
	r, err := installer.Ensure(ctx)
	if err != nil {
		failure = err
		return
	}
	for attempt := 0; attempt < 3; attempt++ {
		port, err := CandidatePort(s.Port)
		if err != nil {
			failure = err
			return
		}
		m.mu.Lock()
		m.port = port
		m.phase = "starting"
		m.mu.Unlock()
		collision, err := m.serve(ctx, installer, r, port)
		if ctx.Err() != nil {
			return
		}
		if !collision {
			failure = err
			return
		}
		m.log.Add("端口在启动期间被占用，正在选择新端口")
	}
	failure = errors.New("连续三次端口竞争，请稍后重试")
}
func (m *Manager) serve(ctx context.Context, i Installer, r Runtime, port int) (bool, error) {
	args := []string{r.CLI, "web", "--no-open", "--port", strconv.Itoa(port)}
	if i.Settings.LAN {
		ip, e := privateAddress()
		if e != nil {
			return false, e
		}
		server, e := lanProxy(ip, port)
		if e != nil {
			return true, e
		}
		defer server.Close()
		args = append(args, "--trusted-host", ip)
		m.mu.Lock()
		m.lanIP = ip
		m.mu.Unlock()
		m.log.Add("局域网访问已启用，仅绑定 " + ip + "；链接持有者拥有完整权限")
	}
	cmd := exec.Command(r.Node, args...)
	cmd.Env = i.environment(r)
	cmd.Dir = m.paths.Data
	prepareProcess(cmd)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}
	cmd.Stderr = cmd.Stdout
	if err = cmd.Start(); err != nil {
		return false, err
	}
	group, err := attachProcess(cmd)
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return false, err
	}
	defer group.close()
	urls := make(chan string, 1)
	scanned := make(chan struct{})
	var collision bool
	go func() {
		defer close(scanned)
		e := ReadLines(out, func(line string) {
			m.log.Add(line)
			if strings.Contains(line, "EADDRINUSE") {
				collision = true
			}
			if u, ok := ParseLaunchURL(line, port); ok {
				select {
				case urls <- u:
				default:
				}
			}
		})
		if e != nil {
			m.log.Add(e.Error())
		}
	}()
	exited := make(chan error, 1)
	go func() { <-scanned; exited <- cmd.Wait() }()
	// Wait is the sole owner of exit; cleanup never enumerates system processes.
	stop := func() {
		group.terminate()
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			group.kill()
			<-exited
		}
	}
	timeout := time.NewTimer(90 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case <-ctx.Done():
			stop()
			return false, nil
		case err := <-exited:
			if err == nil {
				err = errors.New("DSH 服务意外退出")
			}
			return collision, err
		case <-timeout.C:
			stop()
			return false, errors.New("DSH 启动超时；请查看脱敏日志")
		case u := <-urls:
			var ready error
			for tries := 0; tries < 20; tries++ {
				ready = VerifyLaunchURL(ctx, u)
				if ready == nil {
					break
				}
				select {
				case <-ctx.Done():
					stop()
					return false, nil
				case <-time.After(500 * time.Millisecond):
				}
			}
			if ready != nil {
				stop()
				return false, fmt.Errorf("认证就绪检查失败: %w", ready)
			}
			timeout.Stop()
			m.mu.Lock()
			m.launch = u
			m.phase = "running"
			m.mu.Unlock()
			m.log.Add(fmt.Sprintf("已认证并就绪，端口 %d", port))
		}
	}
}
