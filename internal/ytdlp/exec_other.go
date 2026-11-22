//go:build !windows

package ytdlp

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}

func killTree(cmd *exec.Cmd) {} // the default Kill is enough outside Windows

func revealWindows(path string) error { return nil }
