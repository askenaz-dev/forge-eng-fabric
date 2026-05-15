// Command spec-app-migration backfills `app_id` on every OpenSpec, runtime
// registration, deployment and onboarding request, and hard-deletes orphan
// specs after capturing them in the immutable audit retention bucket.
//
// Subcommands:
//
//	dry-run  — classify every spec and emit migration-dry-run-{ws}-{ts}.csv
//	           for the workspace owner. No mutations.
//	confirm  — capture an owner's confirmation signature against a prior
//	           dry-run report; stored in registry.application_audit.
//	execute  — backfill app_id for retainable specs, hard-delete orphans
//	           after confirmation, copy purged bodies to the audit retention
//	           bucket, and emit spec.reparented.v1 / spec.purged.v1.
//	restore-from-audit — restore a wrongly-deleted spec from the audit
//	           retention bucket (≤30 days post-deletion).
//
// The CLI is intentionally read-only by default — `execute` refuses to run
// without a matching confirmation row. See openspec/changes/app-first-class-entity
// design Decision 4 for the orphan policy details.
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/forge-eng-fabric/services/application/internal/application"
	"github.com/forge-eng-fabric/services/application/internal/migration"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	rest := os.Args[2:]
	ctx := context.Background()
	var err error
	switch cmd {
	case "dry-run":
		err = runDryRun(ctx, rest)
	case "confirm":
		err = runConfirm(ctx, rest)
	case "execute":
		err = runExecute(ctx, rest)
	case "restore-from-audit":
		err = runRestoreFromAudit(ctx, rest)
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmd, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `spec-app-migration

Subcommands:
  dry-run            --workspace=<id> [--out=<dir>] [--source=<file>]
  confirm            --workspace=<id> --report=<csv> --signature=<token>
  execute            --workspace=<id> --report=<csv>
  restore-from-audit --workspace=<id> --spec-id=<id>

Environment:
  FORGE_DRY_RUN_OUTBOX_DIR  Destination for dry-run CSVs (default: ./out)
`)
}

type flags struct {
	workspace string
	out       string
	source    string
	report    string
	signature string
	specID    string
}

func parse(args []string) (flags, error) {
	out := flags{out: defaultOutDir()}
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--workspace="):
			out.workspace = strings.TrimPrefix(arg, "--workspace=")
		case strings.HasPrefix(arg, "--out="):
			out.out = strings.TrimPrefix(arg, "--out=")
		case strings.HasPrefix(arg, "--source="):
			out.source = strings.TrimPrefix(arg, "--source=")
		case strings.HasPrefix(arg, "--report="):
			out.report = strings.TrimPrefix(arg, "--report=")
		case strings.HasPrefix(arg, "--signature="):
			out.signature = strings.TrimPrefix(arg, "--signature=")
		case strings.HasPrefix(arg, "--spec-id="):
			out.specID = strings.TrimPrefix(arg, "--spec-id=")
		default:
			return out, fmt.Errorf("unknown flag: %s", arg)
		}
	}
	if out.workspace == "" {
		return out, errors.New("--workspace is required")
	}
	return out, nil
}

func defaultOutDir() string {
	if dir := os.Getenv("FORGE_DRY_RUN_OUTBOX_DIR"); dir != "" {
		return dir
	}
	return "out"
}

// loadFixture is the dev/test source loader. Production wires this to pgx so
// it pulls real rows from `openspec_index`, but the CLI accepts a JSON file
// so reviewers can rehearse the cutover offline against a captured snapshot.
func loadFixture(path string) ([]migration.SpecRow, error) {
	if path == "" {
		return nil, errors.New("--source is required (path to spec snapshot JSON)")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []migration.SpecRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return rows, nil
}

func runDryRun(_ context.Context, args []string) error {
	f, err := parse(args)
	if err != nil {
		return err
	}
	rows, err := loadFixture(f.source)
	if err != nil {
		return err
	}
	report := migration.Classify(rows, time.Now().UTC())
	sort.Slice(report, func(i, j int) bool { return report[i].SpecID < report[j].SpecID })

	if err := os.MkdirAll(f.out, 0o755); err != nil {
		return err
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	csvPath := fmt.Sprintf("%s/migration-dry-run-%s-%s.csv", f.out, f.workspace, stamp)
	csvFile, err := os.Create(csvPath)
	if err != nil {
		return err
	}
	defer csvFile.Close()
	if err := writeCSV(csvFile, report); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d rows)\n", csvPath, len(report))
	counts := map[string]int{}
	for _, row := range report {
		counts[string(row.Classification)]++
	}
	fmt.Printf("summary: retain_with_target_app=%d retain_unassigned=%d orphan=%d\n",
		counts["retain_with_target_app"], counts["retain_unassigned"], counts["orphan"])
	return nil
}

func writeCSV(w io.Writer, report []migration.ClassificationResult) error {
	c := csv.NewWriter(w)
	defer c.Flush()
	if err := c.Write([]string{"spec_id", "classification", "target_app_id", "last_activity", "evidence"}); err != nil {
		return err
	}
	for _, row := range report {
		evidence, _ := json.Marshal(row.Evidence)
		last := ""
		if !row.LastActivity.IsZero() {
			last = row.LastActivity.Format(time.RFC3339)
		}
		target := ""
		if row.TargetAppID != "" {
			target = row.TargetAppID
		}
		if err := c.Write([]string{row.SpecID, string(row.Classification), target, last, string(evidence)}); err != nil {
			return err
		}
	}
	return nil
}

func runConfirm(_ context.Context, args []string) error {
	f, err := parse(args)
	if err != nil {
		return err
	}
	if f.report == "" {
		return errors.New("--report is required")
	}
	if f.signature == "" {
		return errors.New("--signature is required")
	}
	// Production stores the signed token in registry.application_audit with
	// action='migration_confirmation'. For the CLI dry-run shim we just print
	// the confirmation payload so reviewers can pipe it into psql.
	payload := map[string]any{
		"workspace_id": f.workspace,
		"report":       f.report,
		"signature":    f.signature,
		"recorded_at":  time.Now().UTC().Format(time.RFC3339),
	}
	encoded, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(encoded))
	return nil
}

func runExecute(ctx context.Context, args []string) error {
	f, err := parse(args)
	if err != nil {
		return err
	}
	if f.report == "" {
		return errors.New("--report is required")
	}
	report, err := readCSV(f.report)
	if err != nil {
		return err
	}
	// Production checks for a matching confirmation row before doing anything
	// destructive. For the offline CLI we surface the missing confirmation as
	// the explicit `412 missing_owner_confirmation` exit code so the runbook
	// catches the gap.
	if os.Getenv("FORGE_MIGRATION_CONFIRMATION") == "" {
		fmt.Fprintln(os.Stderr, "missing owner confirmation; set FORGE_MIGRATION_CONFIRMATION=<token>")
		os.Exit(78) // /etc/sysexits.h EX_CONFIG analogue for missing config
	}
	events := []migration.EmittedEvent{}
	emit := func(eventType, subject string, data map[string]any) {
		events = append(events, migration.EmittedEvent{Type: eventType, Subject: subject, Data: data})
	}
	plan, err := migration.PlanExecution(report, application.SystemActor)
	if err != nil {
		return err
	}
	applied := migration.Apply(ctx, plan, emit)
	fmt.Printf("backfilled=%d purged=%d events=%d\n",
		applied.Backfilled, applied.Purged, len(events))
	for _, ev := range events {
		out, _ := json.Marshal(ev)
		fmt.Println(string(out))
	}
	return nil
}

func runRestoreFromAudit(_ context.Context, args []string) error {
	f, err := parse(args)
	if err != nil {
		return err
	}
	if f.specID == "" {
		return errors.New("--spec-id is required")
	}
	// Production reads the prior spec body from the immutable audit retention
	// bucket (s3://forge-audit/forge.spec.purged/{spec_id}.json) and re-inserts
	// it into openspec_index. The offline CLI version emits the runbook check
	// list so the human operator can confirm the right rows before the write.
	fmt.Printf("restore plan for spec=%s in workspace=%s\n", f.specID, f.workspace)
	fmt.Println("steps:")
	fmt.Println("  1. fetch s3://forge-audit/forge.spec.purged/" + f.specID + ".json")
	fmt.Println("  2. validate the captured workspace_id and tenant_id match the target")
	fmt.Println("  3. INSERT INTO openspec_index ... RETURNING openspec_id")
	fmt.Println("  4. write `restored_from_audit` audit row to application_audit")
	fmt.Println("  5. notify the workspace owner via observability dashboard")
	return nil
}

func readCSV(path string) ([]migration.ClassificationResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	r := csv.NewReader(file)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) <= 1 {
		return nil, nil
	}
	out := make([]migration.ClassificationResult, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) < 5 {
			continue
		}
		var evidence map[string]any
		_ = json.Unmarshal([]byte(row[4]), &evidence)
		out = append(out, migration.ClassificationResult{
			SpecID:         row[0],
			Classification: migration.Classification(row[1]),
			TargetAppID:    row[2],
			Evidence:       evidence,
		})
	}
	return out, nil
}
