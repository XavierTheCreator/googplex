package ytdlp

import (
	"os/exec"
	"strconv"
	"syscall"
)

const createNoWindow = 0x08000000

// hideWindow stops a console window flashing up for each yt-dlp run.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// killTree makes cancel stop yt-dlp's child process too: the standalone .exe
// unpacks itself and runs as a child, which a plain Kill would leave running.
func killTree(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		k := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		hideWindow(k)
		return k.Run()
	}
}

// revealWindows needs a raw command line: explorer wants /select,"path"
// and does not understand Go's usual argument quoting.
func revealWindows(path string) error {
	cmd := exec.Command("explorer")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer /select,"` + path + `"`}
	return cmd.Start()
}
