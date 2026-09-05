package core

import (
	"os"
	"os/exec"
	"sync"
)

// attachManagedStdin gives non-interactive Node children a valid readable
// standard-input handle without connecting them to user input. Windows GUI
// processes may have no console stdin; pnpm lifecycle workers reject that
// handle before a plugin postinstall can run. Keeping the anonymous pipe's
// writer alive until the child exits preserves a readable, idle stream.
func attachManagedStdin(cmd *exec.Cmd) (started func(), close func(), err error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	cmd.Stdin = reader
	var once sync.Once
	started = func() { _ = reader.Close() }
	close = func() {
		once.Do(func() {
			_ = reader.Close()
			_ = writer.Close()
		})
	}
	return started, close, nil
}
