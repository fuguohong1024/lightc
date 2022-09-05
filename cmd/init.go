package cmd

import (
	"github.com/fuguohong1024/lightc/libexec"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "DO NOT RUN IT DIRECTLY",
	Run: func(_ *cobra.Command, _ []string) {
		if err := libexec.InitProcess(); err != nil {
			logrus.Fatal(err)
		}
	},
}
