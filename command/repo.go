package command

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/cli/cli/git"
	"github.com/cli/cli/internal/ghrepo"
	"github.com/cli/cli/utils"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(repoCmd)
	repoCmd.AddCommand(repoCloneCmd)
	repoCmd.AddCommand(repoCreateCmd)
	repoCreateCmd.Flags().Bool("public", false, "Make the new repository public")
	repoCmd.AddCommand(repoViewCmd)
}

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "View repositories",
	Long: `Work with GitHub repositories.

A repository can be supplied as an argument in any of the following formats:
- "OWNER/REPO"
- by URL, e.g. "https://github.com/OWNER/REPO"`,
}

var repoCloneCmd = &cobra.Command{
	Use:   "clone <repo>",
	Args:  cobra.MinimumNArgs(1),
	Short: "Clone a repository locally",
	Long: `Clone a GitHub repository locally.

To pass 'git clone' options, separate them with '--'.`,
	RunE: repoClone,
}

var repoCreateCmd = &cobra.Command{
	Use:   "create [<name>]",
	Short: "Create a new repository",
	Long:  `Create a new GitHub repository.`,
	RunE:  repoCreate,
}

var repoViewCmd = &cobra.Command{
	Use:   "view [<repo>]",
	Short: "View a repository in the browser",
	Long: `View a GitHub repository in the browser.

With no argument, the repository for the current directory is opened.`,
	RunE: repoView,
}

func repoClone(cmd *cobra.Command, args []string) error {
	cloneURL := args[0]
	if !strings.Contains(cloneURL, ":") {
		cloneURL = fmt.Sprintf("https://github.com/%s.git", cloneURL)
	}

	cloneArgs := []string{"clone"}
	cloneArgs = append(cloneArgs, args[1:]...)
	cloneArgs = append(cloneArgs, cloneURL)

	cloneCmd := git.GitCommand(cloneArgs...)
	cloneCmd.Stdin = os.Stdin
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr
	return utils.PrepareCmd(cloneCmd).Run()
}

func repoCreate(cmd *cobra.Command, args []string) error {
	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		dir, err := git.ToplevelDir()
		if err != nil {
			return err
		}
		name = path.Base(dir)
	}

	visibility := "PRIVATE"
	if isPublic, err := cmd.Flags().GetBool("public"); err == nil && isPublic {
		visibility = "PUBLIC"
	}

	ctx := contextForCommand(cmd)
	client, err := apiClientForContext(ctx)
	if err != nil {
		return err
	}

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"name":       name,
			"visibility": visibility,
		},
	}

	var response struct {
		CreateRepository struct {
			Repository struct {
				URL string
			}
		}
	}

	err = client.GraphQL(`
	mutation($input: CreateRepositoryInput!) {
		createRepository(input: $input) {
			repository {
				url
			}
		}
	}
	`, variables, &response)
	if err != nil {
		return err
	}

	cmd.Println(response.CreateRepository.Repository.URL)
	return nil
}

func repoView(cmd *cobra.Command, args []string) error {
	ctx := contextForCommand(cmd)

	var openURL string
	if len(args) == 0 {
		baseRepo, err := determineBaseRepo(cmd, ctx)
		if err != nil {
			return err
		}
		openURL = fmt.Sprintf("https://github.com/%s", ghrepo.FullName(baseRepo))
	} else {
		repoArg := args[0]
		if strings.HasPrefix(repoArg, "http:/") || strings.HasPrefix(repoArg, "https:/") {
			openURL = repoArg
		} else {
			openURL = fmt.Sprintf("https://github.com/%s", repoArg)
		}
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Opening %s in your browser.\n", displayURL(openURL))
	return utils.OpenInBrowser(openURL)
}