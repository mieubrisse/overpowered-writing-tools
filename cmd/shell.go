package cmd

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"github.com/kurtosis-tech/stacktrace"
	"github.com/spf13/cobra"
)

//go:embed shell-integration.sh.tmpl
var shellTemplateFS embed.FS

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Output shell functions for integration",
	Long:  "Generate shell functions that can be sourced to integrate opwriting with your shell",
	RunE:  shellIntegration,
}

func shellIntegration(cmd *cobra.Command, args []string) error {
	// Read the shell template
	templateContent, err := shellTemplateFS.ReadFile("shell-integration.sh.tmpl")
	if err != nil {
		return stacktrace.Propagate(err, "failed to read shell template")
	}

	// Parse and execute the template
	tmpl, err := template.New("shell").Parse(string(templateContent))
	if err != nil {
		return stacktrace.Propagate(err, "failed to parse shell template")
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, struct {
		BinaryName       string
		WritingDirEnvVar string
	}{
		BinaryName:       "opwriting",
		WritingDirEnvVar: WritingDirEnvVar,
	})
	if err != nil {
		return stacktrace.Propagate(err, "failed to execute shell template")
	}

	fmt.Print(buf.String())
	return nil
}