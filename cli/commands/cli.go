package commands

import "fmt"

// RunUpdateCommand formats an update result for CLI output.
func RunUpdateCommand(svc ActivateAPI, jsonOutput bool, printJSON func(v interface{}) error) error {
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

// RunInstallFileCommand formats an install-file result for CLI output.
func RunInstallFileCommand(svc ActivateAPI, file string, jsonOutput bool, printJSON func(v interface{}) error) error {
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

// RunDiffCommand formats a diff result for CLI output.
func RunDiffCommand(svc ActivateAPI, file string) error {
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

// RunSyncCommand formats a sync result for CLI output.
func RunSyncCommand(svc ActivateAPI, jsonOutput bool, printJSON func(v interface{}) error) error {
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
			fmt.Println("Already up to date.")
		}
	case "updated":
		fmt.Println("Sync complete.")
		for _, f := range result.Updated {
			fmt.Printf("  ✓  %s\n", f)
		}
		for _, f := range result.Skipped {
			fmt.Printf("  ⊘  %s (skipped)\n", f)
		}
	}
	return nil
}
