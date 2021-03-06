package cmd

import (
	"path"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/tinkerbell/actions/pkg/git"
	"github.com/tinkerbell/actions/pkg/img"
)

type buildOptions struct {
	context       string
	containerRepo string
	dryRun        bool
	gitRef        string
	platforms     string
}

var buildOpts = &buildOptions{}

var buildCmd = &cobra.Command{
	Use:   "build [--context .] [--dry-run]",
	Short: "Build and push action container images with changes",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuild(buildOpts)
	},
}

func init() {
	buildCmd.PersistentFlags().StringVar(&buildOpts.context, "context", ".", "base path for the proposals repository in your local file system")
	buildCmd.PersistentFlags().StringVar(&buildOpts.containerRepo, "container-repo", "quay.io/tinkerbell-actions", "repository to push the container images to")
	buildCmd.PersistentFlags().BoolVar(&buildOpts.dryRun, "dry-run", false, "only show the modified actions")
	buildCmd.PersistentFlags().StringVar(&buildOpts.gitRef, "git-ref", "HEAD^@", "the git commit or reference to compare to in the format of HEAD..<commit-id>")
	// FIXME: For some odd reason linux/arm/v6 takes forever to build (> 20min), so I excluded it by default.
	buildCmd.PersistentFlags().StringVar(&buildOpts.platforms, "platforms", "linux/amd64,linux/arm64,linux/arm/v7", "the target os and cpu architecture platforms for the container images")

	rootCmd.AddCommand(buildCmd)
}

func runBuild(opts *buildOptions) error {
	actionsPath := path.Join(opts.context, "actions")

	// Find all modified actions.
	modifiedActions := new([]git.TinkerbellAction)
	if err := git.ModifiedActions(modifiedActions, actionsPath, opts.context, opts.gitRef); err != nil {
		return errors.Wrap(err, "failed to scan for modified actions")
	}

	if len(*modifiedActions) > 0 {
		if buildOpts.dryRun {
			log.Info("The following actions were modified and need to be rebuilt:")
			for _, action := range *modifiedActions {
				log.Info(action.String())
			}
		} else {

			// TODO: Run binfmt_misc to enable building multi-arch images.
			// cat /proc/sys/fs/binfmt_misc/qemu-arm | grep flags == "flags: OCF\n"

			// I am not sure if we should run each action build in a go routine,
			// because buildkit is already massively parallelized.
			for _, action := range *modifiedActions {
				actionContext := path.Join(actionsPath, action.Name, action.Version)
				actionDockerfile := path.Join(actionContext, "Dockerfile")
				actionTag := opts.containerRepo + "/" + action.Name + ":" + action.Version

				// Build the container images for all modified actions with buildkit.
				err := img.Build(&img.BuildConfig{
					Context:    actionContext,
					Dockerfile: actionDockerfile,
					Tag:        actionTag,
					Platforms:  buildOpts.platforms,
					Push:       true,
					NoConsole:  false,
				})
				if err != nil {
					log.Error(err.Error())
					return nil
				}
			}
		}
	} else {
		log.Info("No actions were modified since the provided git reference")
	}

	return nil
}
