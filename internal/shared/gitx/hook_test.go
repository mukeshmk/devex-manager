package gitx

import (
	"reflect"
	"sync"
	"testing"
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
