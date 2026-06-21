package cmd

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/isdg/hr/internal/article"
	"github.com/isdg/hr/internal/meta"
)

var fmtCmd = &cobra.Command{
	Use:   "fmt",
	Short: "Re-sanitize all article frontmatter & sidecar aliases",
	Long: "Walks the vault, rewrites any article frontmatter or " +
		"sidecar whose string fields contain whitespace issues " +
		"(embedded newlines, tabs, control chars, leading or " +
		"trailing whitespace). Body content is preserved as-is.",
	RunE: func(cmd *cobra.Command, args []string) error {
		v, _, err := openActiveVault()
		if err != nil {
			return err
		}
		var scanned, articleChanged, sidecarChanged int
		err = filepath.WalkDir(v.FeedsDir(), func(
			p string, d fs.DirEntry, walkErr error,
		) error {
			if walkErr != nil || d.IsDir() {
				return walkErr
			}
			switch {
			case strings.HasSuffix(p, ".meta.toml"):
				articlePath := strings.TrimSuffix(
					p, ".meta.toml") + ".md"
				changed, err := meta.Fmt(articlePath)
				if err != nil {
					fmt.Printf("error %s: %v\n", p, err)
					return nil
				}
				if changed {
					sidecarChanged++
					fmt.Printf("fmt %s\n", p)
				}
			case strings.HasSuffix(p, ".md"):
				scanned++
				changed, err := article.Fmt(p)
				if err != nil {
					fmt.Printf("error %s: %v\n", p, err)
					return nil
				}
				if changed {
					articleChanged++
					fmt.Printf("fmt %s\n", p)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		fmt.Println()
		fmt.Printf(
			"scanned %d articles; rewrote %d md + %d sidecar\n",
			scanned, articleChanged, sidecarChanged)
		return nil
	},
}

func init() { rootCmd.AddCommand(fmtCmd) }
