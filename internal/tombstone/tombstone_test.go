package tombstone

import (
	"os"
	"path/filepath"
	"testing"
)

func writeArticle(t *testing.T, dir, name string) string {
	t.Helper()
	feedDir := filepath.Join(dir, "feeds", "demo")
	if err := os.MkdirAll(feedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(feedDir, name)
	body := "---\ntitle: T\nfeed: demo\n---\n\n# T\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaSidecar(p), []byte("read = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func metaSidecar(p string) string {
	return p[:len(p)-len(".md")] + ".meta.toml"
}

func TestDeleteRestorePurge(t *testing.T) {
	dir := t.TempDir()
	name := "2020-01-01-t-1a2b3c4d.md"
	p := writeArticle(t, dir, name)
	feedDir := filepath.Dir(p)

	// Delete: files gone, marker present, id reported as deleted.
	mp, err := Delete(nil, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal(".md should be removed")
	}
	if _, err := os.Stat(metaSidecar(p)); !os.IsNotExist(err) {
		t.Fatal("sidecar should be removed")
	}
	if _, err := os.Stat(mp); err != nil {
		t.Fatalf("marker missing: %v", err)
	}
	ids, err := DeletedIDs(feedDir)
	if err != nil || !ids["1a2b3c4d"] {
		t.Fatalf("DeletedIDs=%v err=%v", ids, err)
	}

	// Restore: marker gone, id no longer deleted.
	ok, err := Restore(p)
	if err != nil || !ok {
		t.Fatalf("restore ok=%v err=%v", ok, err)
	}
	if ids, _ := DeletedIDs(feedDir); ids["1a2b3c4d"] {
		t.Fatal("id still deleted after restore")
	}
	if _, err := Restore(p); err != nil {
		t.Fatalf("restore on missing marker should be a no-op, got %v", err)
	}

	// Purge: leaves no marker.
	p = writeArticle(t, dir, name)
	if err := Purge(nil, p); err != nil {
		t.Fatal(err)
	}
	if ids, _ := DeletedIDs(feedDir); len(ids) != 0 {
		t.Fatalf("purge should leave no tombstone, got %v", ids)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("purge should remove the .md")
	}
}
