package supervisor

import (
	"context"
	"sync"
	"time"

	"github.com/deviceplane/agent/pkg/agent/validator"
	"github.com/deviceplane/agent/pkg/agent/variables"
	dpcontext "github.com/deviceplane/agent/pkg/context"
	"github.com/deviceplane/agent/pkg/engine"
	"github.com/deviceplane/agent/pkg/models"
)

type Supervisor struct {
	engine                  engine.Engine
	variables               variables.Interface
	reportApplicationStatus func(ctx *dpcontext.Context, applicationID, currentReleaseID string) error
	reportServiceStatus     func(ctx *dpcontext.Context, applicationID, service string, req models.SetDeviceServiceStatusRequest) error
	reportServiceState      func(ctx *dpcontext.Context, applicationID, service string, req models.SetDeviceServiceStateRequest) error
	validators              []validator.Validator

	applicationIDs         map[string]struct{}
	applicationSupervisors map[string]*ApplicationSupervisor
	once                   sync.Once

	lock   sync.RWMutex
	ctx    context.Context
	cancel func()
}

func NewSupervisor(
	engine engine.Engine,
	variables variables.Interface,
	reportApplicationStatus func(ctx *dpcontext.Context, applicationID, currentReleaseID string) error,
	reportServiceStatus func(ctx *dpcontext.Context, applicationID, service string, req models.SetDeviceServiceStatusRequest) error,
	reportServiceState func(ctx *dpcontext.Context, applicationID, service string, req models.SetDeviceServiceStateRequest) error,
	validators []validator.Validator,
) *Supervisor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Supervisor{
		engine:                  engine,
		variables:               variables,
		reportApplicationStatus: reportApplicationStatus,
		reportServiceStatus:     reportServiceStatus,
		reportServiceState:      reportServiceState,
		validators:              validators,

		applicationIDs:         make(map[string]struct{}),
		applicationSupervisors: make(map[string]*ApplicationSupervisor),

		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Supervisor) Set(bundle models.Bundle, applications []models.FullBundledApplication) {
	applicationIDs := make(map[string]struct{})
	for _, application := range applications {
		s.lock.Lock()
		applicationSupervisor, ok := s.applicationSupervisors[application.Application.ID]
		if !ok {
			applicationSupervisor = NewApplicationSupervisor(
				application.Application.ID,
				s.engine,
				s.variables,
				NewReporter(application.Application.ID, s.reportApplicationStatus, s.reportServiceStatus, s.reportServiceState),
				s.validators,
			)
			s.applicationSupervisors[application.Application.ID] = applicationSupervisor
		}
		applicationSupervisor.Set(bundle, application)
		s.lock.Unlock()

		applicationIDs[application.Application.ID] = struct{}{}
	}

	s.lock.Lock()
	s.applicationIDs = applicationIDs
	s.lock.Unlock()

	s.once.Do(func() {
		go s.applicationSupervisorGC()
		go s.containerGC()
	})
}

func (s *Supervisor) applicationSupervisorGC() {
	ticker := time.NewTicker(defaultTickerFrequency)
	defer ticker.Stop()

	for {
		s.lock.RLock()
		danglingApplicationSupervisors := make(map[string]*ApplicationSupervisor)
		for applicationID, applicationSupervisor := range s.applicationSupervisors {
			if _, ok := s.applicationIDs[applicationID]; !ok {
				danglingApplicationSupervisors[applicationID] = applicationSupervisor
			}
		}
		s.lock.RUnlock()

		for applicationID, applicationSupervisor := range danglingApplicationSupervisors {
			applicationSupervisor.Stop()
			s.lock.Lock()
			delete(s.applicationSupervisors, applicationID)
			s.lock.Unlock()
		}

		select {
		case <-ticker.C:
			continue
		}
	}
}

func (s *Supervisor) containerGC() {
	ticker := time.NewTicker(defaultTickerFrequency)
	defer ticker.Stop()

	for {
		instances, err := containerList(s.ctx, s.engine, map[string]struct{}{
			models.ApplicationLabel: struct{}{},
		}, nil, true)
		if err != nil {
			goto cont
		}

		s.lock.RLock()
		for _, instance := range instances {
			applicationID := instance.Labels[models.ApplicationLabel]
			if _, ok := s.applicationSupervisors[applicationID]; !ok {
				// TODO: this could start many goroutines
				go func(instanceID string) {
					if err = containerStop(s.ctx, s.engine, instanceID); err != nil {
						return
					}
					if err = containerRemove(s.ctx, s.engine, instanceID); err != nil {
						return
					}
				}(instance.ID)
			}
		}
		s.lock.RUnlock()

	cont:
		select {
		case <-ticker.C:
			continue
		}
	}
}
