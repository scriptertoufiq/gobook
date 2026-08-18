package main

import (
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// skipDirs are never walked during a rename — build output and VCS metadata
// must not be rewritten.
var skipDirs = map[string]bool{
	".git": true, "bin": true, "vendor": true, "node_modules": true, "tmp": true,
}

// renameable extensions plus the exact filenames we also scan.
var (
	renameableExts  = map[string]bool{".go": true, ".mod": true, ".md": true, ".yml": true, ".yaml": true}
	renameableNames = map[string]bool{"Makefile": true, "Dockerfile": true}
)

// runRename rewrites the Go module path across the whole project, and
// optionally sets the human-facing APP_NAME. The two are independent: the
// module path is what imports resolve against, APP_NAME is only a label.
func runRename(root, oldModule, newModule, appName string, dryRun bool) error {
	if newModule != "" {
		if err := validateModulePath(newModule); err != nil {
			return err
		}
	}

	if newModule == "" && appName == "" {
		return fmt.Errorf("nothing to do — pass a new module path, -app-name, or both")
	}
	if newModule == oldModule && appName == "" {
		return fmt.Errorf("module path is already %q", oldModule)
	}

	if dryRun {
		fmt.Println("DRY RUN — nothing will be written")
	}

	if newModule != "" && newModule != oldModule {
		fmt.Printf("\nModule path\n  %s\n  -> %s\n\n", oldModule, newModule)

		changed, occurrences, err := rewriteModule(root, oldModule, newModule, dryRun)
		if err != nil {
			return err
		}
		fmt.Printf("\n  %d occurrence(s) in %d file(s)\n", occurrences, changed)
	}

	if appName != "" {
		fmt.Printf("\nApp name -> %q\n\n", appName)
		if err := rewriteAppName(root, appName, dryRun); err != nil {
			return err
		}
	}

	if dryRun {
		fmt.Println("\nRe-run without -dry-run to apply.")
		return nil
	}

	fmt.Println("\nNext:")
	fmt.Println("  go mod tidy")
	fmt.Println("  go build ./...")
	if newModule != "" && newModule != oldModule {
		fmt.Println("\nNote: this renames the module, not the directory on disk.")
		fmt.Println("Rename the folder yourself if you want the two to match.")
	}
	return nil
}

func rewriteModule(root, oldModule, newModule string, dryRun bool) (files, occurrences int, err error) {
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if skipDirs[d.Name()] || (strings.HasPrefix(d.Name(), ".") && path != root) {
				return filepath.SkipDir
			}
			return nil
		}
		if !renameableExts[filepath.Ext(path)] && !renameableNames[d.Name()] {
			return nil
		}

		original, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		count := strings.Count(string(original), oldModule)
		if count == 0 {
			return nil
		}

		updated := strings.ReplaceAll(string(original), oldModule, newModule)

		// Go files must still parse afterwards; catching it here means a bad
		// rename fails loudly instead of leaving a broken tree.
		if filepath.Ext(path) == ".go" {
			formatted, ferr := format.Source([]byte(updated))
			if ferr != nil {
				return fmt.Errorf("%s would not compile after rename: %w", rel(root, path), ferr)
			}
			updated = string(formatted)
		}

		files++
		occurrences += count
		fmt.Printf("  %-52s %d\n", rel(root, path), count)

		if dryRun {
			return nil
		}
		return os.WriteFile(path, []byte(updated), 0o644)
	})

	return files, occurrences, err
}

var appNameDefault = regexp.MustCompile(`(env\("APP_NAME",\s*")[^"]*(")`)

// rewriteAppName updates APP_NAME in the env files and the fallback baked into
// config.go, so a fresh clone without a .env still reports the right name.
func rewriteAppName(root, appName string, dryRun bool) error {
	for _, name := range []string{".env", ".env.example"} {
		path := filepath.Join(root, name)

		original, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue // .env is gitignored and may simply not exist yet
			}
			return err
		}

		lines := strings.Split(string(original), "\n")
		found := false
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "APP_NAME=") {
				lines[i] = "APP_NAME=" + appName
				found = true
				break
			}
		}
		if !found {
			continue
		}

		fmt.Printf("  %s\n", name)
		if dryRun {
			continue
		}
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			return err
		}
	}

	configPath := filepath.Join(root, "config", "config.go")
	original, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	updated := appNameDefault.ReplaceAllString(string(original), "${1}"+appName+"${2}")
	if updated == string(original) {
		return nil
	}

	fmt.Printf("  config/config.go\n")
	if dryRun {
		return nil
	}
	return os.WriteFile(configPath, []byte(updated), 0o644)
}

// validateModulePath rejects the inputs that would produce a go.mod Go refuses
// to load. It is deliberately permissive otherwise: bare names like "shop" are
// legal module paths for a project that is never published.
func validateModulePath(path string) error {
	switch {
	case strings.ContainsAny(path, " \t\"'`\\"):
		return fmt.Errorf("module path %q contains whitespace or quotes", path)
	case strings.HasPrefix(path, "-"):
		return fmt.Errorf("module path %q must not start with '-'", path)
	case strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/"):
		return fmt.Errorf("module path %q must not start or end with '/'", path)
	case strings.Contains(path, ".."):
		return fmt.Errorf("module path %q must not contain '..'", path)
	default:
		return nil
	}
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return r
	}
	return path
}
