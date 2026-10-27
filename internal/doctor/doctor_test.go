package doctor

import (
	"fmt"
	"slices"
	"testing"

	"github.com/isdg/hr/internal/config"
)

func TestCheckConfigUnknownKeys(t *testing.T) {
	cfg := &config.Config{
		Unknown: []string{"feeds.user_agent", "feeds.colour", "autocomit"},
	}
	var got []string
	checkConfig(cfg, func(sev Severity, _, format string, args ...any) {
		got = append(got, string(sev)+" "+fmt.Sprintf(format, args...))
	})
	want := []string{
		`warn "user_agent" is set inside a [[feeds]] entry, ` +
			"so it is ignored; move it above the first [[feeds]]",
		`warn unknown key "feeds.colour" is ignored`,
		`warn unknown key "autocomit" is ignored`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}
