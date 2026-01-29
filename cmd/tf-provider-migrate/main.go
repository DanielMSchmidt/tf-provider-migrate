package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/DanielMSchmidt/tf-provider-migrate/internal/migrate"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "check":
		runCheck(os.Args[2:])
	case "migrate":
		runMigrate(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func runCheck(args []string) {
	flags := flag.NewFlagSet("check", flag.ExitOnError)
	path := flags.String("path", ".", "path to provider repo (default: current directory)")
	registry := flags.String("registry-address", "", "registry address (e.g. registry.terraform.io/org/name)")
	providerName := flags.String("provider-name", "", "provider type name override (default derived)")
	flags.Parse(args)

	opts := migrate.Options{
		Path:            *path,
		RegistryAddress: *registry,
		ProviderName:    *providerName,
	}

	report, err := migrate.Check(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("check OK: %s\n", report.Summary())
}

func runMigrate(args []string) {
	flags := flag.NewFlagSet("migrate", flag.ExitOnError)
	path := flags.String("path", ".", "path to provider repo (default: current directory)")
	registry := flags.String("registry-address", "", "registry address (e.g. registry.terraform.io/org/name)")
	providerName := flags.String("provider-name", "", "provider type name override (default derived)")
	dryRun := flags.Bool("dry-run", false, "show planned changes without writing files")
	vendorMode := flags.String("vendor", "off", "vendor mode: on, off")
	flags.Parse(args)

	opts := migrate.Options{
		Path:            *path,
		RegistryAddress: *registry,
		ProviderName:    *providerName,
		DryRun:          *dryRun,
		VendorMode:      *vendorMode,
	}

	report, err := migrate.Migrate(opts)
	if err != nil {
		if errors.Is(err, migrate.ErrDryRun) {
			fmt.Println(report.Summary())
			return
		}
		fmt.Fprintf(os.Stderr, "migrate failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("migrate OK: %s\n", report.Summary())
}

func usage() {
	fmt.Fprintln(os.Stderr, "tf-provider-migrate")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  tf-provider-migrate check [--path PATH] [--registry-address ADDR] [--provider-name NAME]")
	fmt.Fprintln(os.Stderr, "  tf-provider-migrate migrate [--path PATH] [--registry-address ADDR] [--provider-name NAME] [--dry-run] [--vendor on|off]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  check    validate provider is suitable for migration")
	fmt.Fprintln(os.Stderr, "  migrate  add muxing and framework scaffolding")
}
