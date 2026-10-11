package example

import (
	"context"
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

func ExampleMiningFunction() {

	values := MiningValues{User: "alice", Activity: "mining"}
	if err := score.Add(values, 10); err != nil {
		slog.Error("failed to add score", "error", err)
		return
	}
	if err := score.Add(values, -3); err != nil {
		slog.Error("failed to add score", "error", err)
		return
	}
}

func ExampleMiningValues() {

	score, err := EasyStatistics.GetStatistics[MiningValues]("mining_score")
	if err != nil {
		slog.Error("failed to get statistics", "error", err)
		return
	}

	record, err := score.Query().
		Field("user", "alice").
		GroupInterval(EasyStatistics.GroupForever).
		GroupTime("forever").
		Get()
	if err != nil {
		slog.Error("failed to get record", "error", err)
		return
	}
	if record == nil {
		fmt.Println("No uploaded total yet.")
	} else {
		fmt.Println(record.Counter, record.LastUpdateTime)
	}

}

func StatisticsExampleStart(ctx context.Context) {
	EasyRoutine.SafeGoWithCtx(
		ctx, func(taskCtx context.Context) {
			// Add additional service logic here if needed
			ticker := time.NewTicker(time.Second) // Adjust the interval as needed
			defer ticker.Stop()

			ExampleMiningCreateFunction()

			for {
				select {
				case <-taskCtx.Done():
					return
				case <-ticker.C:
					// Perform periodic task here
					//slog.Info("performing periodic task")
					ExampleMiningFunction()
					ExampleMiningValues()
				}

			}

		}, func(recovered EasyRoutine.Panic, failures int) EasyRoutine.PanicDecision {
			slog.Error("additional service panic", "error", recovered.Value, "stack", string(recovered.Stack))
			return EasyRoutine.NoRetry()
		},
	)
}
