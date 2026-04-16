package saga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/atlaspay/platform/internal/common/logger"
	"github.com/atlaspay/platform/internal/common/metrics"
	"github.com/google/uuid"
)

// ErrCompensated indicates a saga failed in business logic but completed compensation.
var ErrCompensated = errors.New("saga failed and compensated")

// StepStatus represents the status of a saga step
type StepStatus string

const (
	StepPending      StepStatus = "pending"
	StepRunning      StepStatus = "running"
	StepCompleted    StepStatus = "completed"
	StepFailed       StepStatus = "failed"
	StepCompensating StepStatus = "compensating"
	StepCompensated  StepStatus = "compensated"
)

// SagaStatus represents the overall saga status
type SagaStatus string

const (
	SagaRunning      SagaStatus = "running"
	SagaCompleted    SagaStatus = "completed"
	SagaFailed       SagaStatus = "failed"
	SagaCompensating SagaStatus = "compensating"
	SagaCompensated  SagaStatus = "compensated"
)

// Step represents a saga step with action and compensation
type Step struct {
	Name         string
	Action       func(ctx context.Context, data interface{}) error
	Compensation func(ctx context.Context, data interface{}) error
}

// StepLog represents a step execution log
type StepLog struct {
	StepName  string     `json:"step_name"`
	Status    StepStatus `json:"status"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// Saga represents a saga instance
type Saga struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Status      SagaStatus             `json:"status"`
	Data        map[string]interface{} `json:"data"`
	Steps       []Step                 `json:"-"`
	StepLogs    []StepLog              `json:"step_logs"`
	CurrentStep int                    `json:"current_step"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Orchestrator manages saga execution
type Orchestrator struct {
	sagas map[string]*Saga
}

// NewOrchestrator creates a new saga orchestrator
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		sagas: make(map[string]*Saga),
	}
}

// NewSaga creates a new saga
func NewSaga(name string, steps []Step) *Saga {
	return &Saga{
		ID:          uuid.New().String(),
		Name:        name,
		Status:      SagaRunning,
		Data:        make(map[string]interface{}),
		Steps:       steps,
		StepLogs:    make([]StepLog, 0),
		CurrentStep: 0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// Execute runs the saga
func (o *Orchestrator) Execute(ctx context.Context, saga *Saga, initialData interface{}) error {
	o.sagas[saga.ID] = saga
	saga.Data["initial"] = initialData
	startedAt := time.Now()

	logger.Info(ctx).
		Str("saga_id", saga.ID).
		Str("saga_name", saga.Name).
		Int("total_steps", len(saga.Steps)).
		Msg("starting saga execution")

	// Execute steps forward
	for i, step := range saga.Steps {
		saga.CurrentStep = i
		stepLog := StepLog{
			StepName:  step.Name,
			Status:    StepRunning,
			StartedAt: time.Now(),
		}
		saga.StepLogs = append(saga.StepLogs, stepLog)

		logger.Info(ctx).
			Str("saga_id", saga.ID).
			Str("step", step.Name).
			Int("step_index", i+1).
			Int("total_steps", len(saga.Steps)).
			Msg("executing saga step")

		err := step.Action(ctx, saga.Data)
		now := time.Now()
		saga.StepLogs[i].EndedAt = &now

		if err != nil {
			saga.StepLogs[i].Status = StepFailed
			saga.StepLogs[i].Error = err.Error()
			saga.Status = SagaFailed
			saga.UpdatedAt = time.Now()

			logger.Error(ctx).
				Err(err).
				Str("saga_id", saga.ID).
				Str("step", step.Name).
				Msg("saga step failed, starting compensation")

			// Start compensation
			err := o.compensate(ctx, saga, i)
			metrics.RecordSaga(saga.Name, string(saga.Status), time.Since(startedAt))
			return err
		}

		saga.StepLogs[i].Status = StepCompleted
		saga.UpdatedAt = time.Now()

		logger.Info(ctx).
			Str("saga_id", saga.ID).
			Str("step", step.Name).
			Msg("saga step completed")
	}

	saga.Status = SagaCompleted
	saga.UpdatedAt = time.Now()

	logger.Info(ctx).
		Str("saga_id", saga.ID).
		Str("saga_name", saga.Name).
		Msg("saga completed successfully")

	metrics.RecordSaga(saga.Name, string(saga.Status), time.Since(startedAt))
	return nil
}

// compensate runs compensating transactions for failed saga
func (o *Orchestrator) compensate(ctx context.Context, saga *Saga, failedStep int) error {
	saga.Status = SagaCompensating

	logger.Info(ctx).
		Str("saga_id", saga.ID).
		Int("failed_step", failedStep).
		Msg("starting saga compensation")

	// Compensate in reverse order (from failed step - 1 down to 0)
	for i := failedStep - 1; i >= 0; i-- {
		step := saga.Steps[i]
		if step.Compensation == nil {
			continue
		}

		compLog := StepLog{
			StepName:  step.Name + "_compensation",
			Status:    StepCompensating,
			StartedAt: time.Now(),
		}
		saga.StepLogs = append(saga.StepLogs, compLog)

		logger.Info(ctx).
			Str("saga_id", saga.ID).
			Str("step", step.Name).
			Msg("executing compensation")

		metrics.RecordSagaCompensation(saga.Name, step.Name)
		err := step.Compensation(ctx, saga.Data)
		now := time.Now()
		saga.StepLogs[len(saga.StepLogs)-1].EndedAt = &now

		if err != nil {
			saga.StepLogs[len(saga.StepLogs)-1].Status = StepFailed
			saga.StepLogs[len(saga.StepLogs)-1].Error = err.Error()

			logger.Error(ctx).
				Err(err).
				Str("saga_id", saga.ID).
				Str("step", step.Name).
				Msg("compensation failed")

			// Compensation failure - critical error
			return fmt.Errorf("compensation failed for step %s: %w", step.Name, err)
		}

		saga.StepLogs[len(saga.StepLogs)-1].Status = StepCompensated

		logger.Info(ctx).
			Str("saga_id", saga.ID).
			Str("step", step.Name).
			Msg("compensation completed")
	}

	saga.Status = SagaCompensated
	saga.UpdatedAt = time.Now()

	logger.Info(ctx).
		Str("saga_id", saga.ID).
		Msg("saga compensation completed")

	return ErrCompensated
}

// GetSaga retrieves a saga by ID
func (o *Orchestrator) GetSaga(id string) (*Saga, bool) {
	saga, exists := o.sagas[id]
	return saga, exists
}

// ToJSON returns the saga as JSON (for debugging/logging)
func (s *Saga) ToJSON() string {
	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data)
}
