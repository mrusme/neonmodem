package cmd

import (
	"os"

	"github.com/mrusme/gobbs/config"
	"github.com/mrusme/gobbs/system"
	"github.com/spf13/cobra"
)

func init() {
	cmd := connectBase()
	rootCmd.AddCommand(cmd)
}

func connectBase() *cobra.Command {
	var sysType string = ""
	var sysURL string = ""
	var sysConfig map[string]interface{}

	var cmd = &cobra.Command{
		Use:   "connect",
		Short: "Connect to BBS",
		Long:  "Add a new connection to a BBS.",
		Run: func(cmd *cobra.Command, args []string) {
			sysConfig = make(map[string]interface{})
			sys, err := system.New(sysType, &sysConfig, LOG)
			if err != nil {
				LOG.Panicln(err)
			}

			if err := sys.Connect(sysURL); err != nil {
				LOG.Panicln(err)
			}

			CFG.Systems = append(CFG.Systems, config.SystemConfig{
				Type:   sysType,
				Config: sys.GetConfig(),
			})
			if err := CFG.Save(); err != nil {
				LOG.Panicln(err)
			}

			LOG.Infoln("Successfully added new connection!")
			os.Exit(0)
		},
	}

	cmd.
		Flags().
		StringVar(
			&sysType,
			"type",
			"",
			"Type of system to connect to (discourse, lemmy)",
		)
	cmd.MarkFlagRequired("type")

	cmd.
		Flags().
		StringVar(
			&sysURL,
			"url",
			"",
			"URL of system (e.g. https://www.keebtalk.com)",
		)
	cmd.MarkFlagRequired("url")

	return cmd
}
