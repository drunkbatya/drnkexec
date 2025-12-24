package scheduler

import (
    "context"
    "testing"
    "time"

    "github.com/drunkbatya/drnkexec/internal/alerts"
    "github.com/drunkbatya/drnkexec/internal/downtime"
    "github.com/drunkbatya/drnkexec/internal/model"
    "github.com/drunkbatya/drnkexec/internal/nrpeclient"
    "github.com/drunkbatya/drnkexec/internal/state"
    "go.uber.org/zap"
)

type stubClient struct {
    count int
    result nrpeclient.Result
    err error
}

func (s *stubClient) Execute(ctx context.Context, host *model.HostConfig, check *model.CheckConfig) (nrpeclient.Result, error) {
    s.count++
    return s.result, s.err
}

func TestSchedulerSkipsDuringDowntime(t *testing.T) {
    logger, _ := zap.NewProduction()
    defer logger.Sync()
    sugar := logger.Sugar()
    host := &model.HostConfig{Name: "h", Hostname: "h"}
    check := &model.CheckConfig{Name: "c", Command: "cmd", ExecutionTimeoutSec: 1, CheckIntervalSec: 1, RetryIntervalSec: 1, MinFailBeforeAlert: 1, MinSuccessBeforeResolve: 1}
    assignment := model.CheckAssignment{Host: host, Check: check}
    dt := downtime.NewManager()
    _, err := dt.AddHost("h", "host", time.Now().Add(-time.Minute), time.Now().Add(time.Minute))
    if err != nil {
        t.Fatalf("downtime add failed: %v", err)
    }
    stub := &stubClient{result: nrpeclient.Result{Status: nrpeclient.StatusOK}}
    mgr := alerts.NewManager(sugar, nil, time.Minute)
    sched := &Scheduler{logger: sugar, client: stub, alerts: mgr, pinger: nil, state: state.NewManager(&model.Config{}), downtime: dt}
    next := sched.executeCheck(context.Background(), assignment, &assignmentState{})
    if stub.count != 0 {
        t.Fatalf("expected zero executions during downtime")
    }
    if next != time.Second {
        t.Fatalf("unexpected next duration %v", next)
    }
}

func TestSchedulerRunsWhenNoDowntime(t *testing.T) {
    logger, _ := zap.NewProduction()
    defer logger.Sync()
    sugar := logger.Sugar()
    host := &model.HostConfig{Name: "h", Hostname: "h"}
    check := &model.CheckConfig{Name: "c", Command: "cmd", ExecutionTimeoutSec: 1, CheckIntervalSec: 1, RetryIntervalSec: 1, MinFailBeforeAlert: 1, MinSuccessBeforeResolve: 1}
    assignment := model.CheckAssignment{Host: host, Check: check}
    stub := &stubClient{result: nrpeclient.Result{Status: nrpeclient.StatusOK}}
    mgr := alerts.NewManager(sugar, nil, time.Minute)
    sched := &Scheduler{logger: sugar, client: stub, alerts: mgr, pinger: nil, state: state.NewManager(&model.Config{}), downtime: downtime.NewManager()}
    sched.executeCheck(context.Background(), assignment, &assignmentState{})
    if stub.count != 1 {
        t.Fatalf("expected execute to run once")
    }
}
