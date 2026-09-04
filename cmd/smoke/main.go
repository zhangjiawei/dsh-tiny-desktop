// smoke exercises the production installer and supervisor without a GUI. It
// requires an explicit disposable root so tests never touch a user's DSH data.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/zhangjiawei/dsh-tiny-desktop/internal/core"
	"os"
	"time"
)

func main() {
	root := flag.String("root", "", "isolated test directory (required)")
	lan := flag.Bool("lan", false, "also verify opt-in LAN authentication")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "--root is required")
		os.Exit(2)
	}
	p, e := core.NewPaths(*root)
	if e != nil {
		panic(e)
	}
	settings := core.Defaults()
	settings.LAN = *lan
	m := core.NewManager(p, settings)
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
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				err := m.VerifyInstallation(ctx)
				if err == nil && *lan {
					var u string
					u, err = m.ShareURL()
					if err == nil {
						err = core.VerifyLaunchURL(ctx, u)
					}
				}
				cancel()
				m.Stop()
				if err != nil {
					fmt.Fprintln(os.Stderr, core.Redact(err.Error()))
					os.Exit(1)
				}
				fmt.Println("PASS: six registered plugins and real native PTY")
				if *lan {
					fmt.Println("PASS: LAN authority-bound authentication")
				}
				fmt.Println("PASS: owned process stopped")
				return
			}
		}
	}
}
