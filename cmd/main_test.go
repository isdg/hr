package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain points HOME and HR_HOME at a scratch directory, so no test
// can read or write the real ~/.hr or ~/.hrrc.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "hr-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("HOME", home)
	os.Setenv("HR_HOME", filepath.Join(home, ".hr"))
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
