package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/callmejustdodo/notion-link/internal/db"
)

func newSearchCmd() *cobra.Command {
	var (
		dbPath  string
		spaceID string
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Find pages whose title matches a substring (case-insensitive).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer conn.Close()
			pages, err := conn.SearchPages(cmd.Context(), args[0], spaceID, limit)
			if err != nil {
				return err
			}
			if len(pages) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no matches)")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tSPACE\tTITLE")
			ctx := cmd.Context()
			spaceNames := make(map[string]string)
			for _, p := range pages {
				name, ok := spaceNames[p.SpaceID]
				if !ok {
					name = conn.SpaceName(ctx, p.SpaceID)
					if name == "" {
						name = shortID(p.SpaceID)
					}
					spaceNames[p.SpaceID] = name
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", p.ID, name, oneLine(p.Title))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to notion.db.")
	cmd.Flags().StringVar(&spaceID, "space", "", "Restrict to a space id.")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows to print.")
	return cmd
}

func newListCmd() *cobra.Command {
	var (
		dbPath  string
		spaceID string
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pages from notion.db, most recently edited first.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer conn.Close()
			pages, err := conn.SearchPages(cmd.Context(), "", spaceID, limit)
			if err != nil {
				return err
			}
			if len(pages) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no pages)")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tSPACE\tTITLE")
			ctx := cmd.Context()
			spaceNames := make(map[string]string)
			for _, p := range pages {
				name, ok := spaceNames[p.SpaceID]
				if !ok {
					name = conn.SpaceName(ctx, p.SpaceID)
					if name == "" {
						name = shortID(p.SpaceID)
					}
					spaceNames[p.SpaceID] = name
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", p.ID, name, oneLine(p.Title))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to notion.db.")
	cmd.Flags().StringVar(&spaceID, "space", "", "Restrict to a space id.")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max rows to print (0 = unlimited).")
	return cmd
}

func newSpacesCmd() *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "spaces",
		Short: "List the workspaces (spaces) cached in notion.db.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer conn.Close()
			spaces, err := conn.Spaces(cmd.Context())
			if err != nil {
				return err
			}
			if len(spaces) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no spaces)")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tPLAN\tTIER")
			for _, s := range spaces {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.ID, oneLine(s.Name), s.PlanType, s.SubscriptionTier)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to notion.db.")
	return cmd
}

func openDB(dbPath string) (*db.Conn, error) {
	path := dbPath
	if path == "" {
		path = db.DefaultPath()
	}
	return db.Open(path)
}

func shortID(id string) string {
	clean := strings.ReplaceAll(id, "-", "")
	if len(clean) <= 8 {
		return clean
	}
	return clean[:8]
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.TrimSpace(s)
}

