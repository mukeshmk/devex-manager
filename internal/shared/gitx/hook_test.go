package gitx

import (
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCommandHookSeesRuns(t *testing.T) {
	tmp := t.TempDir()
	var got []Command
	SetCommandHook(func(c Command) { got = append(got, c) })
	t.Cleanup(func() { SetCommandHook(nil) })
	if _, err := Run(tmp, "--version"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !reflect.DeepEqual(got[0].Args, []string{"--version"}) || got[0].Err != nil || got[0].Dir != tmp {
		t.Fatalf("hook got %+v", got)
	}
}

func TestCommandHookConcurrent(t *testing.T) {
	tmp := t.TempDir()
	var mu sync.Mutex
	n := 0
	SetCommandHook(func(Command) { mu.Lock(); n++; mu.Unlock() })
	t.Cleanup(func() { SetCommandHook(nil) })
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); Run(tmp, "--version") }()
	}
	wg.Wait()
	if n != 20 {
		t.Fatalf("got %d hook calls, want 20", n)
	}
}

func TestRunQuietNoPrompt(t *testing.T) {
	t.Setenv("GIT_ASKPASS", "")
	var got []Command
	SetCommandHook(func(c Command) { got = append(got, c) })
	t.Cleanup(func() { SetCommandHook(nil) })
	done := make(chan error, 1)
	go func() { _, err := RunQuiet(t.TempDir(), "ls-remote", "https://127.0.0.1:1/x.git"); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("want error")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RunQuiet hung")
	}
	if len(got) != 1 || got[0].Err == nil {
		t.Fatalf("hook got %+v", got)
	}
}

func TestSetQuietMakesRunNonInteractive(t *testing.T) {
	t.Setenv("GIT_TERMINAL_PROMPT", "")
	os.Unsetenv("GIT_TERMINAL_PROMPT")
	dir := t.TempDir()
	// An alias runs a shell under git, so it sees git's env and controlling tty.
	probe := func() (string, error) {
		return Run(dir, "-c", `alias.probe=!printf "%s/" "${GIT_TERMINAL_PROMPT-unset}"; (: </dev/tty) 2>/dev/null && echo tty || echo notty`, "probe")
	}
	if out, err := probe(); err != nil || !strings.HasPrefix(out, "unset/") {
		t.Fatalf("quiet off must leave the env alone: %q %v", out, err)
	}
	SetQuiet(true)
	t.Cleanup(func() { SetQuiet(false) })
	want := "0/notty"
	if runtime.GOOS == "windows" {
		want = "0/"
	}
	if out, err := probe(); err != nil || !strings.HasPrefix(out, want) {
		t.Fatalf("quiet on: %q %v, want %q", out, err, want)
	}
	done := make(chan error, 1)
	go func() { _, err := Run(t.TempDir(), "ls-remote", "https://127.0.0.1:1/x.git"); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("want error")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run hung")
	}
}
