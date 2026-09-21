package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/LazyEasyDev/LZApp/app"
	"github.com/LazyEasyDev/LZApp/app/db"
	easylog "github.com/LazyEasyDev/LZApp/components/easy_log"
	config "github.com/LazyEasyDev/LZApp/config"
	urfavecli "github.com/urfave/cli/v3"
)

func Run(ctx context.Context, args []string) error {
	return (&urfavecli.Command{
		Name:           "lzapp",
		Usage:          "manage the LZApp service",
		DefaultCommand: "start",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{
				Name:     "config",
				Usage:    "select configuration profile (debug or release)",
				Value:    string(config.ProfileRelease),
				OnlyOnce: true,
				Validator: func(value string) error {
					switch config.Profile(value) {
					case config.ProfileDebug, config.ProfileRelease:
						return nil
					default:
						return fmt.Errorf("must be debug or release")
					}
				},
				Action: func(_ context.Context, _ *urfavecli.Command, value string) error {
					config.InitConfig(config.Profile(value))
					return nil
				},
			},
		},
		ArgValidator: func(_ context.Context, command *urfavecli.Command) error {
			if command.Name != "help" && command.Args().Present() {
				var usage strings.Builder
				urfavecli.HelpPrinter(&usage, urfavecli.RootCommandHelpTemplate, command.Root())
				return fmt.Errorf("%s accepts no positional arguments\n\n%s", command.FullName(), usage.String())
			}
			return nil
		},
		Commands: []*urfavecli.Command{
			{
				Name:  "start",
				Usage: "start the LZApp service",
				Flags: []urfavecli.Flag{
					&urfavecli.IntFlag{
						Name:     "https-port",
						Usage:    "override the selected configuration's HTTPS port",
						OnlyOnce: true,
						Validator: func(port int) error {
							if port < 1 || port > 65535 {
								return fmt.Errorf("must be between 1 and 65535")
							}
							return nil
						},
						Action: func(_ context.Context, command *urfavecli.Command, port int) error {
							config.InitConfig(config.Profile(command.String("config")))
							config.GetConfig().HTTP.HTTPSPort = port
							return nil
						},
					},
				},
				Action: func(ctx context.Context, _ *urfavecli.Command) error {
					return app.Run(ctx)
				},
			},
			{
				Name:  "db",
				Usage: "manage the database",
				Commands: []*urfavecli.Command{
					{
						Name:  "init",
						Usage: "create tables and initialize data",
						Action: func(ctx context.Context, cmd *urfavecli.Command) error {
							return db.Run(ctx)
						},
					},
				},
			},
			{
				Name:  "showlog",
				Usage: "show recent application logs",
				Flags: []urfavecli.Flag{
					&urfavecli.StringFlag{
						Name:     "level",
						Usage:    "only show records at this level",
						OnlyOnce: true,
						Validator: func(level string) error {
							switch level {
							case "debug", "info", "warn", "err":
								return nil
							default:
								return fmt.Errorf("must be debug, info, warn, or err")
							}
						},
					},
					&urfavecli.IntFlag{
						Name:     "tail",
						Usage:    "number of recent records to show",
						OnlyOnce: true,
						Validator: func(tail int) error {
							if tail < 1 {
								return fmt.Errorf("must be positive")
							}
							return nil
						},
					},
				},

				Action: func(_ context.Context, command *urfavecli.Command) error {
					return easylog.ShowLogs(command.Root().Writer, command.String("level"), command.Int("tail"))
				},
			},
		},
	}).Run(ctx, args)
}
