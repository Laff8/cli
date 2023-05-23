package delete

import (
	"strconv"

	"github.com/cli/cli/v2/internal/tableprinter"
	"github.com/cli/cli/v2/pkg/cmd/project/shared/format"
	"github.com/cli/cli/v2/pkg/cmd/project/shared/queries"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/cobra"
)

type deleteOpts struct {
	userOwner string
	orgOwner  string
	number    int32
	projectID string
	format    string
}

type deleteConfig struct {
	tp     *tableprinter.TablePrinter
	client *api.GraphQLClient
	opts   deleteOpts
}

type deleteProjectMutation struct {
	DeleteProject struct {
		Project queries.Project `graphql:"projectV2"`
	} `graphql:"deleteProjectV2(input:$input)"`
}

func NewCmdDelete(f *cmdutil.Factory, runF func(config deleteConfig) error) *cobra.Command {
	opts := deleteOpts{}
	deleteCmd := &cobra.Command{
		Short: "Delete a project",
		Use:   "delete [<number>]",
		Example: `
# delete the current user's project 1
gh project delete 1 --user "@me"

# delete user monalisa's project 1
gh project delete 1 --user monalisa

# delete org github's project 1
gh project delete 1 --org github
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.MutuallyExclusive(
				"only one of `--user` or `--org` may be used",
				opts.userOwner != "",
				opts.orgOwner != "",
			); err != nil {
				return err
			}

			client, err := queries.NewClient()
			if err != nil {
				return err
			}

			if len(args) == 1 {
				num, err := strconv.ParseInt(args[0], 10, 32)
				if err != nil {
					return cmdutil.FlagErrorf("invalid number: %v", args[0])
				}
				opts.number = int32(num)
			}

			t := tableprinter.New(f.IOStreams)
			config := deleteConfig{
				tp:     t,
				client: client,
				opts:   opts,
			}

			// allow testing of the command without actually running it
			if runF != nil {
				return runF(config)
			}
			return runDelete(config)
		},
	}

	deleteCmd.Flags().StringVar(&opts.userOwner, "user", "", "Login of the user owner. Use \"@me\" for the current user.")
	deleteCmd.Flags().StringVar(&opts.orgOwner, "org", "", "Login of the organization owner")
	cmdutil.StringEnumFlag(deleteCmd, &opts.format, "format", "", "", []string{"json"}, "Output format")

	return deleteCmd
}

func runDelete(config deleteConfig) error {
	owner, err := queries.NewOwner(config.client, config.opts.userOwner, config.opts.orgOwner)
	if err != nil {
		return err
	}

	project, err := queries.NewProject(config.client, owner, config.opts.number, false)
	if err != nil {
		return err
	}
	config.opts.projectID = project.ID

	query, variables := deleteItemArgs(config)
	err = config.client.Mutate("DeleteProject", query, variables)
	if err != nil {
		return err
	}

	if config.opts.format == "json" {
		return printJSON(config, *project)
	}

	return printResults(config)

}

func deleteItemArgs(config deleteConfig) (*deleteProjectMutation, map[string]interface{}) {
	return &deleteProjectMutation{}, map[string]interface{}{
		"input": githubv4.DeleteProjectV2Input{
			ProjectID: githubv4.ID(config.opts.projectID),
		},
		"firstItems":  githubv4.Int(0),
		"afterItems":  (*githubv4.String)(nil),
		"firstFields": githubv4.Int(0),
		"afterFields": (*githubv4.String)(nil),
	}
}

func printResults(config deleteConfig) error {
	// using table printer here for consistency in case it ends up being needed in the future
	config.tp.AddField("Deleted project")
	config.tp.EndRow()
	return config.tp.Render()
}

func printJSON(config deleteConfig, project queries.Project) error {
	b, err := format.JSONProject(project)
	if err != nil {
		return err
	}
	config.tp.AddField(string(b))
	return config.tp.Render()
}
