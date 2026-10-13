package example

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/LazyEasyDev/EasyRoutine"
	"github.com/LazyEasyDev/EasyStatistics"
)

type MiningValues struct {
	User     string `stat:"user"`
	Activity string `stat:"activity"`
}

var score *EasyStatistics.Statistics[MiningValues]

func ExampleMiningCreateFunction() {
	var err error
	score, err = EasyStatistics.NewStatistics[MiningValues](
		"mining_score",
		time.Minute,
		[]EasyStatistics.Dimension{
			{
				OrderFields:    []string{"user", "activity"},
				GroupIntervals: []EasyStatistics.GroupInterval{EasyStatistics.GroupMonth, EasyStatistics.GroupForever},
				ClearAfter:     90 * 24 * time.Hour,
			},
			{
				OrderFields:    []string{"user"},
				GroupIntervals: []EasyStatistics.GroupInterval{EasyStatistics.GroupForever},
			},
		},
	)
	if err != nil {
		slog.Error("failed to create statistics", "error", err)
		return
	}
}

func ExampleMiningAddFunction(values *MiningValues, scoreValue int64) error {
	if values == nil {
		return errors.New("values is nil")
	}

	if score == nil {
		return errors.New("statistics not initialized")
	}

	if err := score.Add(*values, scoreValue); err != nil {
		return fmt.Errorf("failed to add score: %w", err)
	}

	return nil
}

func ExampleMiningValues() {

	if score == nil {
		slog.Error("statistics not initialized")
		return
	}

	records, err := score.Query().
		Field("user", "alice").
		GroupInterval(EasyStatistics.GroupForever).
		GroupTime("forever").
		Get()
	if err != nil {
		slog.Error("failed to get record", "error", err)
		return
	}
	if len(records) == 0 {
		fmt.Println("No uploaded total yet.")
	} else {
		fmt.Println(records[0].Counter, records[0].LastUpdateTime)
	}

}

func StatisticsExampleStart(ctx context.Context) {

	ExampleMiningCreateFunction()

	EasyRoutine.SafeGoWithCtx(
		ctx, func(taskCtx context.Context) {
			// Add additional service logic here if needed
			ticker := time.NewTicker(time.Second) // Adjust the interval as needed
			defer ticker.Stop()

			for {
				select {
				case <-taskCtx.Done():
					return
				case <-ticker.C:
					// Perform periodic task here
					ExampleMiningAddFunction(&MiningValues{User: "alice", Activity: "mining"}, 10)
					ExampleMiningValues()
				}

			}

		}, func(recovered EasyRoutine.Panic, failures int) EasyRoutine.PanicDecision {
			slog.Error("additional service panic", "error", recovered.Value, "stack", string(recovered.Stack))
			return EasyRoutine.NoRetry()
		},
	)
}
