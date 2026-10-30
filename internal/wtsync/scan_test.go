package wtsync

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindConflictMarkers(t *testing.T) {
	main := t.TempDir()
	put(t, main+"/.vscode/settings.json.devex-conflict-feat", "x") // TEST 5b: inside a copy path
	put(t, main+"/.env.devex-conflict-feat", "x")                  // next to a file copy path
	put(t, main+"/.claude/file.devex-conflict-feat", "x")          // TEST 5c: symlink-only path
	put(t, main+"/node_modules/a.devex-conflict-feat", "x")        // outside copy paths
	got := FindConflictMarkers(main, []string{".vscode", ".env", "missing"})
	want := []string{main + "/.env.devex-conflict-feat", main + "/.vscode/settings.json.devex-conflict-feat"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := FindConflictMarkers(main, []string{"node_modules/x"}); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}

func TestScan(t *testing.T) {
	root := t.TempDir()
	main, feat := root+"/main", root+"/feat wt"
	put(t, main+"/my conf/a b.json", "12345")
	put(t, feat+"/my conf/sub/c.json", "c")
	put(t, feat+"/my conf/a b.json.devex-conflict-x", "skip")
	put(t, feat+"/.env", "E=1")
	put(t, root+"/elsewhere/linked.json", "no")
	if err := os.Symlink(root+"/elsewhere", main+"/my conf/link"); err != nil {
		t.Fatal(err)
	}
	when := time.Unix(1700000000, 0)
	if err := os.Chtimes(main+"/my conf/a b.json", when, when); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]Stat{".env": s(5, 3)}
	files, err := Scan(main, []string{feat}, []string{"my conf", ".env", "absent"}, manifest)
	if err != nil {
		t.Fatal(err)
	}
	var rels []string
	for _, f := range files {
		rels = append(rels, f.Rel)
	}
	if want := []string{".env", "my conf/a b.json", "my conf/sub/c.json"}; !reflect.DeepEqual(rels, want) {
		t.Fatalf("rels %q want %q", rels, want)
	}
	if f := files[0]; f.Recorded != s(5, 3) || f.Main.Exists || f.Others[feat] != (Stat{true, f.Others[feat].Mtime, 3}) {
		t.Fatalf(".env %+v", f)
	}
	if f := files[1]; f.Main != s(1700000000, 5) || f.Others[feat].Exists || f.Recorded.Exists {
		t.Fatalf("a b.json %+v", f)
	}
}

func TestManifestRoundTrip(t *testing.T) {
	path := t.TempDir() + "/info/devex/sync-manifest"
	if m, err := ReadManifest(path); err != nil || len(m) != 0 {
		t.Fatalf("missing manifest: %v %v", m, err)
	}
	in := map[string]Stat{"b|odd.json": s(2, 20), "a b/x": s(1, 10)}
	if err := WriteManifest(path, in); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "a b/x|1|10\nb|odd.json|2|20\n" {
		t.Fatalf("manifest %q", data)
	}
	if err := os.WriteFile(path, append(data, "junk\nx|y|z\n|1|2\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadManifest(path)
	if err != nil || !reflect.DeepEqual(got, in) {
		t.Fatalf("got %v %v", got, err)
	}
}
