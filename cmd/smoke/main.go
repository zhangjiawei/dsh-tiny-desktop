// smoke exercises the production installer and supervisor without a GUI. It
// requires an explicit disposable root so tests never touch a user's DSH data.
package main

import (
	"flag"
	"fmt"
	"github.com/zhangjiawei/dsh-tiny-desktop/internal/core"
	"os"
	"time"
)

func main() {
	root := flag.String("root", "", "isolated test directory (required)")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "--root is required")
		os.Exit(2)
	}
	p, e := core.NewPaths(*root)
	if e != nil {
		panic(e)
	}
	m := core.NewManager(p, core.Defaults())
	m.Start()
	last := 0
	deadline := time.After(20 * time.Minute)
	for {
		select {
		case <-deadline:
			m.Stop()
			fmt.Fprintln(os.Stderr, "timeout")
			os.Exit(1)
		case <-time.After(time.Second):
			s := m.Snapshot()
			for _, l := range s.Logs[last:] {
				fmt.Println(l.Time, l.Text)
			}
			last = len(s.Logs)
			if s.Phase == "error" {
				fmt.Fprintln(os.Stderr, s.Error)
				os.Exit(1)
			}
			if s.Phase == "running" {
				fmt.Println("PASS: authenticated DSH startup on", s.Port)
				m.Stop()
				fmt.Println("PASS: owned process stopped")
				return
			}
		}
	}
}
