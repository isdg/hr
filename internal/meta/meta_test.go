package meta

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func article(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "2020-01-01-a-aaaaaaaa.md")
	if err := os.WriteFile(p, []byte("# a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func exists(t *testing.T, p string) bool {
	t.Helper()
	_, err := os.Stat(p)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

// Every field must count, or a sidecar holding only that field would be
// deleted as default.
func TestIsZeroCoversEveryField(t *testing.T) {
	if !(&Meta{}).IsZero() {
		t.Fatal("zero Meta is not IsZero")
	}
	now := time.Now()
	typ := reflect.TypeFor[Meta]()
	for i := range typ.NumField() {
		var m Meta
		f := reflect.ValueOf(&m).Elem().Field(i)
		switch f.Kind() {
		case reflect.Bool:
			f.SetBool(true)
		case reflect.String:
			f.SetString("x")
		case reflect.Pointer:
			f.Set(reflect.ValueOf(&now))
		case reflect.Slice:
			f.Set(reflect.MakeSlice(f.Type(), 1, 1))
		default:
			t.Fatalf("field %s: unhandled kind %s", typ.Field(i).Name, f.Kind())
		}
		if m.IsZero() {
			t.Errorf("IsZero ignores %s", typ.Field(i).Name)
		}
	}
}

func TestSaveZeroRemovesSidecar(t *testing.T) {
	p := article(t)
	if err := Save(p, &Meta{}); err != nil {
		t.Fatal(err)
	}
	if exists(t, Path(p)) {
		t.Error("saving defaults wrote a sidecar")
	}
	if err := MarkRead(p); err != nil {
		t.Fatal(err)
	}
	if !exists(t, Path(p)) {
		t.Fatal("MarkRead wrote no sidecar")
	}
	if err := MarkUnread(p); err != nil {
		t.Fatal(err)
	}
	if exists(t, Path(p)) {
		t.Error("back to defaults, the sidecar should be gone")
	}
	if m := LoadOrDefault(p); !m.IsZero() {
		t.Errorf("absent sidecar loads as %+v", m)
	}
}

func TestFmtRemovesDefaultSidecar(t *testing.T) {
	p := article(t)
	err := os.WriteFile(Path(p), []byte("read = false\nfavorite = false\n"),
		0o644)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := Fmt(p)
	if err != nil || !changed {
		t.Fatalf("Fmt = %v, %v; want a change", changed, err)
	}
	if exists(t, Path(p)) {
		t.Error("default-only sidecar survived Fmt")
	}

	if _, err := ToggleFavorite(p); err != nil {
		t.Fatal(err)
	}
	if changed, err := Fmt(p); err != nil || changed {
		t.Errorf("Fmt on a favorite = %v, %v; want no change", changed, err)
	}
}
