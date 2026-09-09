package core

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// processMarker is intentionally private runtime metadata. It lets a newly
// launched Tiny recover a DSH child left behind by a crashed Tiny process
// without enumerating or matching unrelated system processes.
type processMarker struct {
	PID        int    `json:"pid"`
	OwnerPID   int    `json:"ownerPid"`
	Executable string `json:"executable"`
}

func processMarkerPath(paths Paths) string {
	return filepath.Join(paths.Root, ".dsh-process.json")
}

func (m *Manager) recoverOrphan() {
	path := processMarkerPath(m.paths)
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		m.log.Add("无法读取上次 DSH 运行标记：" + err.Error())
		return
	}
	var marker processMarker
	if err = json.Unmarshal(contents, &marker); err != nil || marker.PID <= 0 {
		_ = os.Remove(path)
		return
	}
	// A second Tiny instance must never terminate the active owner's DSH child.
	// Still clean up older abandoned children: a newly started Tiny may have
	// selected another port, which would otherwise make the old child invisible
	// once the marker is replaced by the new process.
	owners := map[int]bool{os.Getpid(): true}
	ownerAlive := marker.OwnerPID > 0 && processAlive(marker.OwnerPID)
	if ownerAlive {
		owners[marker.OwnerPID] = true
	}
	if !ownerAlive && terminateMarkedProcess(marker.PID, marker.Executable, m.paths.Data) {
		m.log.Add("已清理上次异常退出遗留的 DSH 服务")
	}
	for _, pid := range cleanupManagedOrphans(m.paths, owners) {
		if pid != marker.PID {
			m.log.Add("已清理旧 Tiny 实例遗留的 DSH 服务")
		}
	}
	// A transient second-instance process must not erase the active owner's
	// recovery marker. The Wails single-instance callback will wake that owner;
	// its normal shutdown remains responsible for clearing the marker.
	if ownerAlive {
		return
	}
	_ = os.Remove(path)
}

func (m *Manager) writeProcessMarker(pid int, executable string) error {
	contents, err := json.Marshal(processMarker{PID: pid, OwnerPID: os.Getpid(), Executable: executable})
	if err != nil {
		return err
	}
	return AtomicWrite(processMarkerPath(m.paths), contents, 0600)
}

func (m *Manager) clearProcessMarker(pid int) {
	path := processMarkerPath(m.paths)
	contents, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var marker processMarker
	if json.Unmarshal(contents, &marker) == nil && marker.PID == pid && marker.OwnerPID == os.Getpid() {
		_ = os.Remove(path)
	}
}
