package cli

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/LazyEasyDev/EasyRoutine"
	"github.com/LazyEasyDev/LZApp/components"
	config "github.com/LazyEasyDev/LZApp/config"
	urfavecli "github.com/urfave/cli/v3"
)

func Run(ctx context.Context, args []string) error {

	return (&urfavecli.Command{
		Name:  "lzapp",
		Usage: "manage the LZApp service",
		Before: func(
			ctx context.Context,
			command *urfavecli.Command,
		) (context.Context, error) {
			config.InitConfig(config.Profile(command.String("config")))
			return ctx, nil
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
				Action: func(_ context.Context, command *urfavecli.Command, port int) error {
					config.GetConfig().HTTP.HTTPSPort = port
					return nil
				},
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
				Name:  "db",
				Usage: "manage the database",
				Commands: []*urfavecli.Command{
					{
						Name:   "init",
						Usage:  "create tables and initialize data",
						Action: cmd_actions,
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

	//before action execution first initialize the components
	fundemental_init_err := components.InitFundemental(config.GetConfig())
	if fundemental_init_err != nil {
		return fmt.Errorf("initialize fundemental components: %w", fundemental_init_err)
	}

	err := components.Init(ctx, config.GetConfig())
	if err != nil {
		slog.Error("failed to initialize components", "error", err)
		return fmt.Errorf("initialize components: %w", err)
	}

	// execute the command action
	slog.Info("executing command", "command", cmd.Name)

	EasyRoutine.SafeGo(
		ctx,
		func(ctx context.Context) {
			<-ctx.Done()
			slog.Info("safeGo END!")
		},
		func(recovered EasyRoutine.Panic, failures int) EasyRoutine.PanicDecision {
			log.Printf("attempt %d panicked: %v\n%s", failures, recovered.Value, recovered.Stack)
			if failures >= 3 {
				return EasyRoutine.NoRetry()
			}
			return EasyRoutine.PanicDecision{
				Retry: true,
				After: 30 * time.Second,
			}
		},
	)
	/////
	components.Close()
	return nil
}
