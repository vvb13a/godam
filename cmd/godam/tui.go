package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/vvb13a/godam/internal/delivery/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive Terminal UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(
			tui.NewModel(assetService, collectionService),
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)
		_, err := p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
