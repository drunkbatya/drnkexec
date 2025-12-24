package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/drunkbatya/drnkexec/internal/alerts"
	"github.com/drunkbatya/drnkexec/internal/downtime"
	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/nrpeclient"
	"github.com/drunkbatya/drnkexec/internal/pinger"
	"github.com/drunkbatya/drnkexec/internal/state"
	"go.uber.org/zap"
)

type Scheduler struct {
	logger   *zap.SugaredLogger
	client   nrpeclient.Client
	alerts   *alerts.Manager
	pinger   pinger.Checker
	state    *state.Manager
	downtime *downtime.Manager
}

func NewScheduler(logger *zap.SugaredLogger, client nrpeclient.Client, alerts *alerts.Manager, pinger pinger.Checker, state *state.Manager, downtime *downtime.Manager) *Scheduler {
	return &Scheduler{logger: logger, client: client, alerts: alerts, pinger: pinger, state: state, downtime: downtime}
}

// Run starts background goroutines for every check assignment.
func (s *Scheduler) Run(ctx context.Context, cfg *model.Config) {
	var wg sync.WaitGroup
	for _, assignment := range cfg.LookupMaps.CheckAssignments {
		assign := assignment
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.runAssignment(ctx, assign)
		}()
	}

	<-ctx.Done()
	s.logger.Infof("scheduler stopping")
	wg.Wait()
}

type assignmentState struct {
	consecutiveFailures int
	consecutiveSuccess  int
	alertActive         bool
}

func (s *Scheduler) runAssignment(ctx context.Context, assignment model.CheckAssignment) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	state := &assignmentState{}

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			next := s.executeCheck(ctx, assignment, state)
			if next <= 0 {
				next = time.Duration(assignment.Check.CheckIntervalSec) * time.Second
			}
			timer.Reset(next)
		}
	}
}

func (s *Scheduler) executeCheck(ctx context.Context, assignment model.CheckAssignment, state *assignmentState) time.Duration {
	if s.inDowntime(assignment) {
		s.logger.Infof("downtime active, skipping execution host=%s check=%s", assignment.Host.Hostname, assignment.Check.Name)
		return secondsToDuration(assignment.Check.CheckIntervalSec)
	}
	timeout := secondsToDuration(assignment.Check.ExecutionTimeoutSec)
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := s.execute(checkCtx, assignment)
	success := err == nil && result.Status == nrpeclient.StatusOK
	output := result.Output
	if output == "" && err != nil {
		output = err.Error()
	}
	statusForState := result.Status
	if err != nil && statusForState == nrpeclient.StatusOK {
		statusForState = nrpeclient.StatusUnknown
	}
	if s.state != nil {
		s.state.Update(assignment, statusForState, output)
	}

	if success {
		state.consecutiveSuccess++
		state.consecutiveFailures = 0
		s.logger.Debugf("check ok host=%s check=%s output=%s", assignment.Host.Hostname, assignment.Check.Name, output)
		if state.alertActive && state.consecutiveSuccess >= assignment.Check.MinSuccessBeforeResolve {
			if !s.inDowntime(assignment) {
				s.alerts.Resolve(ctx, assignment, output)
				state.alertActive = false
			} else {
				s.logger.Infof("check resolve suppressed due to downtime host=%s check=%s", assignment.Host.Hostname, assignment.Check.Name)
			}
		}
		return secondsToDuration(assignment.Check.CheckIntervalSec)
	}

	state.consecutiveFailures++
	state.consecutiveSuccess = 0
	s.logger.Warnf("check failed host=%s check=%s output=%s error=%v", assignment.Host.Hostname, assignment.Check.Name, output, err)

	if state.consecutiveFailures < assignment.Check.MinFailBeforeAlert {
		return secondsToDuration(assignment.Check.RetryIntervalSec)
	}

	if s.inDowntime(assignment) {
		s.logger.Infof("alert suppressed due to downtime host=%s check=%s", assignment.Host.Hostname, assignment.Check.Name)
		return secondsToDuration(assignment.Check.RepeatAlertIntervalSec)
	}
	if !state.alertActive {
		state.alertActive = true
	}
	s.alerts.Alert(ctx, assignment, output)

	return secondsToDuration(assignment.Check.RepeatAlertIntervalSec)
}

func (s *Scheduler) inDowntime(assignment model.CheckAssignment) bool {
	if s.downtime == nil {
		return false
	}
	return s.downtime.Active(assignment.Host.Hostname, assignment.Check.Name, time.Now())
}

func (s *Scheduler) execute(ctx context.Context, assignment model.CheckAssignment) (nrpeclient.Result, error) {
	switch assignment.Check.Type {
	case model.CheckTypePing:
		return s.executePing(ctx, assignment)
	case model.CheckTypeNRPE, "":
		return s.client.Execute(ctx, assignment.Host, assignment.Check)
	default:
		return nrpeclient.Result{}, fmt.Errorf("unsupported check type %s", assignment.Check.Type)
	}
}

func (s *Scheduler) executePing(ctx context.Context, assignment model.CheckAssignment) (nrpeclient.Result, error) {
	if s.pinger == nil {
		return nrpeclient.Result{}, fmt.Errorf("ping checker not configured")
	}
	output, err := s.pinger.Ping(ctx, assignment.Host)
	if err != nil {
		return nrpeclient.Result{Output: output, Status: nrpeclient.StatusCritical}, err
	}
	return nrpeclient.Result{Output: output, Status: nrpeclient.StatusOK}, nil
}

func secondsToDuration(value int) time.Duration {
	if value <= 0 {
		return time.Second
	}
	return time.Duration(value) * time.Second
}
