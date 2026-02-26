package main

import "fmt"

// CLI output formatters for non-interactive command execution.

func runUpdateCommand(svc *ActivateService, jsonOutput bool) error {
	result, err := svc.Update()
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(result)
	}

	for _, f := range result.Updated {
		fmt.Printf("  ✓  %s\n", f)
	}
	for _, f := range result.Skipped {
		fmt.Printf("  ⊘  %s (skipped)\n", f)
	}
	fmt.Printf("\nUpdated %d files, skipped %d.\n", len(result.Updated), len(result.Skipped))
	return nil
}

func runInstallFileCommand(svc *ActivateService, file string, jsonOutput bool) error {
	result, err := svc.InstallFile(file)
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(result)
	}
	fmt.Printf("  ✓  %s\n", result.File)
	return nil
}

func runDiffCommand(svc *ActivateService, file string) error {
	result, err := svc.DiffFile(file)
	if err != nil {
		return err
	}

	if result.Identical {
		fmt.Println("Files are identical.")
	} else {
		fmt.Print(result.Diff)
	}
	return nil
}

func runSyncCommand(svc *ActivateService, jsonOutput bool) error {
	result, err := svc.Sync()
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(result)
	}

	switch result.Action {
	case "none":
		if result.Reason == "not installed" {
			fmt.Println("Not installed. Run 'repo add' first.")
		} else {
			fmt.Printf("Already up to date (v%s).\n", result.AvailableVersion)
		}
	case "updated":
		fmt.Printf("Updated from v%s to v%s.\n", result.PreviousVersion, result.AvailableVersion)
		for _, f := range result.Updated {
			fmt.Printf("  ✓  %s\n", f)
		}
		for _, f := range result.Skipped {
			fmt.Printf("  ⊘  %s (skipped)\n", f)
		}
	}
	return nil
}
