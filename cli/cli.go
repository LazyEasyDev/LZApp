package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/LazyEasyDev/LZApp/app"
	"github.com/LazyEasyDev/LZApp/app/db"
	"github.com/LazyEasyDev/LZApp/components"
	config "github.com/LazyEasyDev/LZApp/config"
	urfavecli "github.com/urfave/cli/v3"
)

func Run(ctx context.Context, args []string) error {

	return (&urfavecli.Command{
		Name:           "lzapp",
		Usage:          "manage the LZApp service",
		DefaultCommand: "start",
		Before: func(
			ctx context.Context,
			command *urfavecli.Command,
		) (context.Context, error) {
			// initialize configuration based on the selected profile
			config.InitConfig(config.Profile(command.String("config")))

			if command.IsSet("https-port") {
				config.GetConfig().HTTP.HTTPSPort = command.Int("https-port")
			}

			//before action execution initialize the components first
			newCtx := context.WithValue(ctx, "cli_initialize_started", true)
			init_err := components.Init(ctx, config.GetConfig())
			if init_err != nil {
				return newCtx, init_err
			} else {
				return newCtx, nil
			}
		},
		After: func(ctx context.Context, command *urfavecli.Command) error {
			// after action execution,
			// wait for all routines to complete and close components
			if ctx.Value("cli_initialize_started") == true {
				return components.WaitAndClose()
			} else {
				return nil
			}
		},
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{
				Name:     "config",
				Usage:    "select configuration profile (debug or release)",
				Value:    "release",
				OnlyOnce: true,
				Validator: func(value string) error {
					switch config.Profile(value) {
					case config.ProfileDebug, config.ProfileRelease:
						return nil
					default:
						return fmt.Errorf("must be debug or release")
					}
				},
			},
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
			},
		},
		Commands: []*urfavecli.Command{
			{
				Name:  "start",
				Usage: "start the LZApp service",
				Action: func(ctx context.Context, _ *urfavecli.Command) error {
					return app.Start(ctx)
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
							return db.Init(ctx, components.GetComponents().DB)
						},
					},
				},
			},
			{
				Name:      "showlog",
				Usage:     "show recent application logs",
				ArgsUsage: "[level=debug|info|warn|err]",
				Flags: []urfavecli.Flag{
					&urfavecli.StringFlag{
						Name:     "level",
						Usage:    "only show records at this level",
						OnlyOnce: true,
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
				Action: cmd_actions,
			},
		},
	}).Run(ctx, args)
}

func cmd_actions(ctx context.Context, cmd *urfavecli.Command) error {
	// this is just currently a test function
	// execute the command action
	slog.Info("executing command", "command", cmd.Name)

	return nil
}
