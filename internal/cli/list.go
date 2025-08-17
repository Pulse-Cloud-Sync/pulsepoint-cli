package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List synced files and directories",
	Long: `Display a list of files and directories that are being synced
or have been synced by PulsePoint.`,
	RunE: runList,
}

func init() {
	listCmd.Flags().String("path", "", "Filter by path")
	listCmd.Flags().String("status", "", "Filter by status (synced, pending, error)")
	listCmd.Flags().Bool("remote", false, "List remote files")
	listCmd.Flags().Bool("tree", false, "Display as tree structure")
	listCmd.Flags().Int("limit", 50, "Limit number of results")
	listCmd.Flags().String("sort", "name", "Sort by: name, size, date")
}

func runList(cmd *cobra.Command, args []string) error {
	path, _ := cmd.Flags().GetString("path")
	status, _ := cmd.Flags().GetString("status")
	remote, _ := cmd.Flags().GetBool("remote")
	tree, _ := cmd.Flags().GetBool("tree")
	limit, _ := cmd.Flags().GetInt("limit")
	sortBy, _ := cmd.Flags().GetString("sort")

	// Header
	if remote {
		fmt.Printf("☁️  Remote Files (Google Drive)\n")
	} else {
		fmt.Printf("💾 Local Files\n")
	}
	fmt.Printf("═══════════════════════════════════════\n\n")

	// Filter information
	if path != "" {
		fmt.Printf("📁 Path: %s\n", path)
	}
	if status != "" {
		fmt.Printf("🔍 Status: %s\n", status)
	}
	fmt.Printf("📊 Sort: %s | Limit: %d\n\n", sortBy, limit)

	if tree {
		// Tree view
		fmt.Println("📁 /Users/user/Documents")
		fmt.Println("├── 📄 report.pdf (2.3 MB) ✅")
		fmt.Println("├── 📄 data.xlsx (1.1 MB) ✅")
		fmt.Println("├── 📁 Projects")
		fmt.Println("│   ├── 📄 proposal.docx (845 KB) ✅")
		fmt.Println("│   ├── 📄 budget.xlsx (512 KB) ⏳")
		fmt.Println("│   └── 📁 Images")
		fmt.Println("│       ├── 🖼️ logo.png (125 KB) ✅")
		fmt.Println("│       └── 🖼️ banner.jpg (256 KB) ✅")
		fmt.Println("└── 📄 notes.txt (12 KB) ✅")
	} else {
		// List view
		fmt.Printf("%-40s %-10s %-20s %-10s\n", "Name", "Size", "Modified", "Status")
		fmt.Printf("%-40s %-10s %-20s %-10s\n", "────", "────", "────────", "──────")

		// Sample data
		files := []struct {
			name     string
			size     string
			modified string
			status   string
		}{
			{"report.pdf", "2.3 MB", time.Now().Add(-1 * time.Hour).Format("2006-01-02 15:04"), "✅ Synced"},
			{"data.xlsx", "1.1 MB", time.Now().Add(-2 * time.Hour).Format("2006-01-02 15:04"), "✅ Synced"},
			{"proposal.docx", "845 KB", time.Now().Add(-3 * time.Hour).Format("2006-01-02 15:04"), "✅ Synced"},
			{"budget.xlsx", "512 KB", time.Now().Add(-30 * time.Minute).Format("2006-01-02 15:04"), "⏳ Pending"},
			{"notes.txt", "12 KB", time.Now().Add(-4 * time.Hour).Format("2006-01-02 15:04"), "✅ Synced"},
		}

		for _, file := range files {
			fmt.Printf("%-40s %-10s %-20s %-10s\n", file.name, file.size, file.modified, file.status)
		}
	}

	// Summary
	fmt.Printf("\n")
	fmt.Printf("═══════════════════════════════════════\n")
	fmt.Printf("📊 Summary: 5 files, 4.8 MB total\n")
	fmt.Printf("   ✅ Synced: 4 | ⏳ Pending: 1 | ❌ Errors: 0\n")

	return nil
}
