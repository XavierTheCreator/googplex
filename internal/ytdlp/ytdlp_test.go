package ytdlp

import (
	"slices"
	"testing"
)

func TestParseLine(t *testing.T) {
	cases := []struct {
		line string
		ok   bool
		want Progress
	}{
		{"GP   12.3% 00:41", true, Progress{Status: "downloading", Percent: 12.3, ETA: "00:41"}},
		{"GP  0.0% Unknown", true, Progress{Status: "downloading"}},
		{"GP 100.0% 00:00", true, Progress{Status: "merging", Percent: 100}},
		{"GP NA% NA", false, Progress{}},
		{"GPDONE /Users/me/Downloads/Googleplex/A video [abcdefghijk].mp4", true,
			Progress{Status: "done", Percent: 100, Path: "/Users/me/Downloads/Googleplex/A video [abcdefghijk].mp4"}},
		{"[youtube] abcdefghijk: Downloading webpage", false, Progress{}},
		{"", false, Progress{}},
	}
	for _, c := range cases {
		got, ok := ParseLine(c.line)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseLine(%q) = %+v, %v; want %+v, %v", c.line, got, ok, c.want, c.ok)
		}
	}
}

func TestArgs(t *testing.T) {
	with := Args("abcdefghijk", "/out", "/opt/homebrew/bin/ffmpeg")
	if !slices.Contains(with, "--merge-output-format") || !slices.Contains(with, "/opt/homebrew/bin/ffmpeg") {
		t.Errorf("ffmpeg args missing: %v", with)
	}
	without := Args("abcdefghijk", "/out", "")
	if slices.Contains(without, "--merge-output-format") || !slices.Contains(without, "b[ext=mp4]/b") {
		t.Errorf("no-ffmpeg args wrong: %v", without)
	}
	if without[len(without)-1] != "https://www.youtube.com/watch?v=abcdefghijk" {
		t.Errorf("url should be last: %v", without)
	}
}

func TestFindSum(t *testing.T) {
	sums := "aaa  yt-dlp\nBBB  yt-dlp_macos\nccc  yt-dlp.exe\n"
	if s, err := findSum(sums, "yt-dlp_macos"); err != nil || s != "bbb" {
		t.Errorf("findSum = %q, %v", s, err)
	}
	if _, err := findSum(sums, "yt-dlp_linux"); err == nil {
		t.Error("missing asset should fail")
	}
}

func TestDownloadRejectsBadID(t *testing.T) {
	if err := Download(t.Context(), "../../etc", t.TempDir(), func(Progress) {}); err == nil {
		t.Error("bad id should fail before running anything")
	}
}

func TestAsset(t *testing.T) {
	if _, err := asset(); err != nil {
		t.Error(err)
	}
}
