// Command make scaffolds a resource across every layer and wires it into the
// container, routes and migration registry.
//
//	go run ./cmd/make scaffold Category    # all files + wiring
//	go run ./cmd/make model Category       # just the model + migration entry
//	go run ./cmd/make controller Category  # just the controller + wiring
//
// It also renames the project:
//
//	go run ./cmd/make rename github.com/you/shop -app-name "Shop API"
//
// And it writes migrations:
//
//	go run ./cmd/make migration add_status_to_posts
//
// Sub-commands: scaffold, model, migration, repository, service, controller,
// request, resource, test, rename. Run with -h for the full flag list.
//
// Wiring works by inserting above `// codegen:` marker comments in
// internal/container/container.go and internal/routes/api.go. Don't delete
// those markers. Migrations need no marker — each file registers itself from
// an init function, so writing it is the whole of the installation.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	command := os.Args[1]
	if command == "-h" || command == "--help" || command == "help" {
		usage()
		return
	}

	fs := flag.NewFlagSet(command, flag.ExitOnError)
	force := fs.Bool("force", false, "overwrite files that already exist")
	noWire := fs.Bool("no-wire", false, "generate files without touching container/routes/migration")
	appName := fs.String("app-name", "", "rename: set APP_NAME in the env files and config default")
	dryRun := fs.Bool("dry-run", false, "rename: report what would change without writing")
	fs.Usage = usage

	args := parseInterleaved(fs, os.Args[2:])

	root, module, err := findModule()
	if err != nil {
		fail(err)
	}

	if command == "rename" {
		var newModule string
		if len(args) > 0 {
			newModule = args[0]
		}
		if err := runRename(root, module, newModule, *appName, *dryRun); err != nil {
			fail(err)
		}
		return
	}

	if len(args) < 1 {
		usage()
		os.Exit(2)
	}

	names := newNames(args[0], module)
	if names.Pascal == "" {
		fail(fmt.Errorf("%q is not a usable resource name", args[0]))
	}

	// Migrations are ordered by ID, so the stamp is what fixes a new one after
	// everything that already exists. Seconds resolution is plenty: two
	// migrations created in the same second would need the same author running
	// the generator twice in one breath, and the duplicate-ID panic catches it.
	stamp := time.Now().Format("20060102150405")
	if command == "migration" {
		// A free-form description of a change: `add_status_to_posts`.
		names.MigrationID = stamp + "_" + names.Snake
	} else {
		// A resource is being scaffolded, so the migration creates its table.
		names.MigrationID = stamp + "_create_" + names.PluralSnake + "_table"
	}

	steps, err := stepsFor(command, names)
	if err != nil {
		fail(err)
	}

	fmt.Printf("Scaffolding %s in %s\n\n", names.Pascal, root)

	var wirings []wiring
	for _, s := range steps {
		written, err := writeFile(filepath.Join(root, s.path), s.tmpl, names, *force)
		if err != nil {
			fail(err)
		}

		if written {
			fmt.Printf("  created  %s\n", s.path)
			wirings = append(wirings, s.wirings...)
			continue
		}
		fmt.Printf("  skipped  %s (already exists, use -force to overwrite)\n", s.path)
	}

	if *noWire || len(wirings) == 0 {
		printNext(names, command)
		return
	}

	fmt.Println()
	for _, w := range wirings {
		applied, err := applyWiring(root, w, names)
		if err != nil {
			// A failed insert isn't fatal — print the snippet so it can be
			// pasted by hand rather than losing the generated files.
			fmt.Printf("  !! could not wire %s: %v\n", w.file, err)
			fmt.Printf("     add this manually above `%s`:\n%s\n", w.marker, indentBlock(render(w.block, names), "       "))
			continue
		}

		if applied {
			fmt.Printf("  wired    %s (%s)\n", w.file, w.marker)
			continue
		}
		fmt.Printf("  wired    %s (%s) — already present, unchanged\n", w.file, w.marker)
	}

	printNext(names, command)
}

// parseInterleaved lets flags appear anywhere after the sub-command by peeling
// positional arguments off one at a time. Go's flag package otherwise stops at
// the first non-flag, silently ignoring `... scaffold Category -force`.
func parseInterleaved(fs *flag.FlagSet, argv []string) []string {
	var positional []string

	for len(argv) > 0 {
		if err := fs.Parse(argv); err != nil {
			os.Exit(2)
		}

		argv = fs.Args()
		if len(argv) == 0 {
			break
		}
		positional = append(positional, argv[0])
		argv = argv[1:]
	}

	return positional
}

// step is one generated file plus the wiring it implies. Splitting wiring per
// step means generating layers one at a time composes to the same result as
// running `scaffold`.
type step struct {
	path    string
	tmpl    string
	wirings []wiring
}

type wiring struct {
	file   string
	marker string
	block  string
}

const (
	containerFile = "internal/container/container.go"
	routesFile    = "internal/routes/api.go"
)

func stepsFor(command string, n Names) ([]step, error) {
	// No wiring: migrations register themselves from an init function, so the
	// file being on disk is the whole of the installation.
	migrationStep := step{
		path: fmt.Sprintf("internal/database/migrations/%s.go", n.MigrationID),
		tmpl: migrationTemplate,
	}

	modelStep := step{
		path: fmt.Sprintf("internal/models/%s.go", n.Snake),
		tmpl: modelTemplate,
	}

	// The migration that comes with a model builds its table from the struct,
	// rather than being the blank one `make migration` writes.
	createTableStep := step{
		path: migrationStep.path,
		tmpl: createTableTemplate,
	}

	repositoryStep := step{
		path: fmt.Sprintf("internal/repositories/%s_repository.go", n.Snake),
		tmpl: repositoryTemplate,
		wirings: []wiring{{
			file:   containerFile,
			marker: "codegen:repositories",
			block:  "{{.Camel}}Repo := repositories.New{{.Pascal}}Repository(db)",
		}},
	}

	serviceStep := step{
		path: fmt.Sprintf("internal/services/%s_service.go", n.Snake),
		tmpl: serviceTemplate,
		wirings: []wiring{{
			file:   containerFile,
			marker: "codegen:services",
			block:  "{{.Camel}}Service := services.New{{.Pascal}}Service({{.Camel}}Repo)",
		}},
	}

	controllerStep := step{
		path: fmt.Sprintf("internal/controllers/%s_controller.go", n.Snake),
		tmpl: controllerTemplate,
		wirings: []wiring{
			{
				file:   containerFile,
				marker: "codegen:fields",
				block:  "{{.Pascal}} *controllers.{{.Pascal}}Controller",
			},
			{
				file:   containerFile,
				marker: "codegen:controllers",
				block:  "{{.Pascal}}: controllers.New{{.Pascal}}Controller({{.Camel}}Service),",
			},
			{
				file:   routesFile,
				marker: "codegen:routes",
				block: `// Protected by default. Delete authenticated/verified to make
// {{.PluralSnake}} public, rather than remembering to add them.
{{.PluralCamel}} := api.Group("/{{.PluralKebab}}", authenticated, verified)
{
	{{.PluralCamel}}.GET("", c.{{.Pascal}}.Index)
	{{.PluralCamel}}.POST("", c.{{.Pascal}}.Store)
	{{.PluralCamel}}.GET("/:id", c.{{.Pascal}}.Show)
	{{.PluralCamel}}.PATCH("/:id", c.{{.Pascal}}.Update)
	{{.PluralCamel}}.PUT("/:id", c.{{.Pascal}}.Update)
	{{.PluralCamel}}.DELETE("/:id", c.{{.Pascal}}.Destroy)
}
`,
			},
		},
	}

	requestStep := step{
		path: fmt.Sprintf("internal/requests/%s_request.go", n.Snake),
		tmpl: requestTemplate,
	}
	resourceStep := step{
		path: fmt.Sprintf("internal/resources/%s_resource.go", n.Snake),
		tmpl: resourceTemplate,
	}
	testStep := step{
		path: fmt.Sprintf("internal/services/%s_service_test.go", n.Snake),
		tmpl: serviceTestTemplate,
	}

	switch command {
	case "scaffold", "all":
		return []step{modelStep, createTableStep, repositoryStep, requestStep, resourceStep, serviceStep, controllerStep, testStep}, nil
	case "model":
		return []step{modelStep, createTableStep}, nil
	case "migration":
		return []step{migrationStep}, nil
	case "repository", "repo":
		return []step{repositoryStep}, nil
	case "service":
		return []step{serviceStep}, nil
	case "controller":
		return []step{controllerStep}, nil
	case "request":
		return []step{requestStep}, nil
	case "resource":
		return []step{resourceStep}, nil
	case "test":
		return []step{testStep}, nil
	default:
		return nil, fmt.Errorf("unknown sub-command %q (see: go run ./cmd/make -h)", command)
	}
}

// writeFile renders a template and formats it. Reports false when the file
// already exists and force is off.
func writeFile(path, tmpl string, n Names, force bool) (bool, error) {
	if _, err := os.Stat(path); err == nil && !force {
		return false, nil
	}

	source, err := format.Source([]byte(render(tmpl, n)))
	if err != nil {
		return false, fmt.Errorf("formatting %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, source, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// applyWiring inserts a block directly above a `// codegen:<marker>` line,
// matching that line's indentation. Reports false when the block is already
// present, so re-running is safe.
func applyWiring(root string, w wiring, n Names) (bool, error) {
	path := filepath.Join(root, w.file)

	original, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	block := render(w.block, n)
	firstLine := strings.TrimSpace(strings.SplitN(block, "\n", 2)[0])
	if strings.Contains(string(original), firstLine) {
		return false, nil
	}

	var (
		out      []string
		inserted bool
	)

	scanner := bufio.NewScanner(strings.NewReader(string(original)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if !inserted && strings.Contains(line, w.marker) {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			out = append(out, strings.Split(strings.TrimRight(indentBlock(block, indent), "\n"), "\n")...)
			inserted = true
		}
		out = append(out, line)
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}

	if !inserted {
		return false, fmt.Errorf("marker %q not found", w.marker)
	}

	source, err := format.Source([]byte(strings.Join(out, "\n") + "\n"))
	if err != nil {
		return false, fmt.Errorf("result would not compile: %w", err)
	}

	return true, os.WriteFile(path, source, 0o644)
}

// render executes a template, translating `~` into a backtick so the templates
// can contain struct tags.
func render(tmpl string, n Names) string {
	t, err := template.New("gen").Parse(strings.ReplaceAll(tmpl, "~", "`"))
	if err != nil {
		fail(err)
	}

	var buf strings.Builder
	if err := t.Execute(&buf, n); err != nil {
		fail(err)
	}
	return buf.String()
}

func indentBlock(block, indent string) string {
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}

// findModule walks up from the working directory to locate go.mod, returning
// the project root and the module path so generated imports are always correct.
func findModule() (root, module string, err error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", "", err
	}

	for {
		candidate := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(candidate); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if after, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
					return dir, strings.TrimSpace(after), nil
				}
			}
			return "", "", fmt.Errorf("no module directive in %s", candidate)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", fmt.Errorf("go.mod not found — run this from inside the project")
		}
		dir = parent
	}
}

func printNext(n Names, command string) {
	fmt.Printf("\nNext:\n")

	switch command {
	case "scaffold", "all", "model":
		fmt.Printf("  1. edit internal/models/%s.go — replace the placeholder fields\n", n.Snake)
		fmt.Printf("  2. go run ./cmd/migrate        — create the %s table\n", n.PluralSnake)
		fmt.Printf("  3. make run                    — try GET /api/v1/%s\n", n.PluralKebab)
	case "migration":
		fmt.Printf("  1. fill in Up and Down in internal/database/migrations/%s.go\n", n.MigrationID)
		fmt.Printf("  2. go run ./cmd/migrate -status — confirm it shows as pending\n")
		fmt.Printf("  3. go run ./cmd/migrate         — apply it\n")
	default:
		fmt.Printf("  go build ./... && make run\n")
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage: go run ./cmd/make <sub-command> [name] [flags]

Generate:
  scaffold     model + migration + repository + request + resource + service + controller + test
  model        internal/models/<name>.go            (+ a create-table migration)
  migration    internal/database/migrations/<timestamp>_<name>.go
  repository   internal/repositories/<name>_repository.go
  service      internal/services/<name>_service.go
  controller   internal/controllers/<name>_controller.go  (+ container + routes)
  request      internal/requests/<name>_request.go
  resource     internal/resources/<name>_resource.go
  test         internal/services/<name>_service_test.go

Project:
  rename       change the Go module path across every file, and/or APP_NAME

Flags:
  -force       overwrite files that already exist
  -no-wire     generate files without editing container/routes/migration
  -app-name    rename: the human-facing name (APP_NAME, /health, startup log)
  -dry-run     rename: show what would change without writing

Examples:
  go run ./cmd/make scaffold Category
  go run ./cmd/make scaffold blog_post
  go run ./cmd/make model Comment
  go run ./cmd/make migration add_status_to_posts

  go run ./cmd/make rename github.com/you/shop -dry-run
  go run ./cmd/make rename github.com/you/shop -app-name "Shop API"
  go run ./cmd/make rename -app-name "Shop API"       # label only, imports untouched
`)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "make: %v\n", err)
	os.Exit(1)
}
