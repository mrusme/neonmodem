package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/mrusme/neonmodem/config"
	"github.com/mrusme/neonmodem/system"
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
		PreRun: func(cmd *cobra.Command, args []string) {
			sysType, _ := cmd.Flags().GetString("type")
			sysType = strings.ToLower(sysType)
			if sysType != "hackernews" {
				cmd.MarkFlagRequired("url")
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			sysConfig = make(map[string]interface{})
			sys, err := system.New(sysType, &sysConfig, LOG)
			if err != nil {
				LOG.Panicln(err)
			}

			sysURLparsed, err := url.Parse(sysURL)
			if err != nil {
				fmt.Print(err)
				os.Exit(1)
			}

			if caps := sys.GetCapabilities(); !caps.IsCapableOf("connect:multiple") {
				for _, existingSys := range CFG.Systems {
					if existingSys.Type == sysType {
						existingSysURL, ok := existingSys.Config["url"]
						if !ok {
							fmt.Println("Cannot add multiple instances of this system!")
							os.Exit(1)
						}

						existingSysURLparsed, err := url.Parse(existingSysURL.(string))
						if err != nil {
							fmt.Print(err)
							os.Exit(1)
						}

						//&& existingSysURLparsed.RequestURI() == sysURLparsed.RequestURI()
						if existingSysURLparsed.Host == sysURLparsed.Host {
							fmt.Println("Cannot add multiple instances of this system!")
							os.Exit(1)
						}
					}
				}
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

			fmt.Println("Successfully added new connection!")
			os.Exit(0)
		},
	}

	cmd.
		Flags().
		StringVar(
			&sysType,
			"type",
			"",
			"Type of system to connect to (discourse, lemmy, lobsters, hackernews)",
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

	return cmd
}
