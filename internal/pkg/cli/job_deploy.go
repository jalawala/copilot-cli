// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/term/log"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/command"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

type deployJobVars struct {
	appName      string
	name         string
	envName      string
	imageTag     string
	resourceTags map[string]string
}

type deployJobOpts struct {
	deployJobVars

	store        store
	ws           wsJobDirReader
	unmarshal    func(in []byte) (interface{}, error)
	cmd          runner
	sessProvider sessionProvider

	spinner progress
	sel     wsSelector
	prompt  prompter
}

func newJobDeployOpts(vars deployJobVars) (*deployJobOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	prompter := prompt.New()
	return &deployJobOpts{
		deployJobVars: vars,

		store:        store,
		ws:           ws,
		unmarshal:    manifest.UnmarshalWorkload,
		spinner:      termprogress.NewSpinner(),
		sel:          selector.NewWorkspaceSelect(prompter, store, ws),
		prompt:       prompter,
		cmd:          command.New(),
		sessProvider: sessions.NewProvider(),
	}, nil
}

// Validate returns an error if the user inputs are invalid.
func (o *deployJobOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if o.name != "" {
		if err := o.validateJobName(); err != nil {
			return err
		}
	}
	if o.envName != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any required fields that are not provided.
func (o *deployJobOpts) Ask() error {
	if err := o.askJobName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	if err := o.askImageTag(); err != nil {
		return err
	}
	return nil
}

// Execute builds and pushes the container image for the job.
func (o *deployJobOpts) Execute() error {
	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *deployJobOpts) RecommendedActions() []string {
	return nil
}

func (o *deployJobOpts) validateJobName() error {
	names, err := o.ws.JobNames()
	if err != nil {
		return fmt.Errorf("list jobs in the workspace: %w", err)
	}
	for _, name := range names {
		if o.name == name {
			return nil
		}
	}
	return fmt.Errorf("job %s not found in the workspace", color.HighlightUserInput(o.name))
}

func (o *deployJobOpts) validateEnvName() error {
	_, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return fmt.Errorf("get environment %s configuration: %w", o.envName, err)
	}
	return nil
}

func (o *deployJobOpts) askJobName() error {
	if o.name != "" {
		return nil
	}

	name, err := o.sel.Job("Select a job from your workspace", "")
	if err != nil {
		return fmt.Errorf("select job: %w", err)
	}
	o.name = name
	return nil
}

func (o *deployJobOpts) askEnvName() error {
	if o.envName != "" {
		return nil
	}

	name, err := o.sel.Environment("Select an environment", "", o.appName)
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = name
	return nil
}

func (o *deployJobOpts) askImageTag() error {
	if o.imageTag != "" {
		return nil
	}

	tag, err := getVersionTag(o.cmd)

	if err == nil {
		o.imageTag = tag

		return nil
	}

	log.Warningln("Failed to default tag, are you in a git repository?")

	userInputTag, err := o.prompt.Get(inputImageTagPrompt, "", prompt.RequireNonEmpty)
	if err != nil {
		return fmt.Errorf("prompt for image tag: %w", err)
	}
	o.imageTag = userInputTag
	return nil
}

// buildJobDeployCmd builds the `job deploy` subcommand.
func buildJobDeployCmd() *cobra.Command {
	vars := deployJobVars{}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys a job to an environment.",
		Long:  `Deploys a job to an environment.`,
		Example: `
  Deploys a job named "report-gen" to a "test" environment.
  /code $ copilot job deploy --name report-gen --env test
  Deploys a job with additional resource tags.
  /code $ copilot job deploy --resource-tags source/revision=bb133e7,deployment/initiator=manual`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newJobDeployOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Execute(); err != nil {
				return err
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)

	return cmd
}
