package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List hosts in a formatted table",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := database.Open(sshcmConfigDir())
		if err != nil {
			return fmt.Errorf("opening database: %w", err)
		}
		defer db.Close()

		hosts, err := models.ListHosts(db.SQL, models.HostFilter{})
		if err != nil {
			return fmt.Errorf("listing hosts: %w", err)
		}
		if len(hosts) == 0 {
			fmt.Println("No host entries found.")
			return nil
		}

		tagMap, _ := models.GetAllTags(db.SQL, "host")

		printTable(hosts, tagMap)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func printTable(hosts []models.Host, tagMap map[int64][]string) {
	headers := []string{"Host", "Description", "HostName", "Tags"}

	rows := make([][]string, len(hosts))
	for i, h := range hosts {
		tags := "-"
		if t := tagMap[h.ID]; len(t) > 0 {
			var lines []string
			for j := 0; j < len(t); j += 2 {
				if j+1 < len(t) {
					lines = append(lines, t[j]+", "+t[j+1])
				} else {
					lines = append(lines, t[j])
				}
			}
			tags = strings.Join(lines, "\n")
		}
		desc := h.Description
		if desc == "" {
			desc = "-"
		}
		rows[i] = []string{h.Nickname, desc, h.Hostname, tags}
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})

	fmt.Fprintln(os.Stdout, t)
}
