package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fragpit/env-cleaner/internal/model"
	"github.com/fragpit/env-cleaner/pkg/utils"
)

type EnvironmentService struct {
	repo              model.Repository
	connectorFactory  ConnectorFactory
	maxExtendDuration string
}

func NewEnvironmentService(
	repo model.Repository,
	connectorFactory ConnectorFactory,
	maxExtendDuration string,
) *EnvironmentService {
	return &EnvironmentService{
		repo:              repo,
		connectorFactory:  connectorFactory,
		maxExtendDuration: maxExtendDuration,
	}
}

func (s *EnvironmentService) GetEnvironments(
	ctx context.Context,
) ([]*model.Environment, error) {
	return s.repo.GetEnvironments(ctx)
}

func (s *EnvironmentService) AddEnvironment(
	ctx context.Context,
	env *model.Environment,
	ttl string,
) error {
	conn, err := s.connectorFactory.GetConnector(env.Type)
	if err != nil {
		return &model.ValidationError{
			Msg: fmt.Sprintf("error getting connector: %v", err),
		}
	}

	deleteAt, deleteAtSec, err := utils.SetDeleteAt(ttl)
	if err != nil {
		return &model.ValidationError{
			Msg: fmt.Sprintf("error setting delete ttl: %v", err),
		}
	}

	env.DeleteAt = deleteAt
	env.DeleteAtSec = deleteAtSec

	env.EnvID, err = conn.GetEnvironmentID(ctx, env)
	if err != nil {
		return &model.ValidationError{
			Msg: fmt.Sprintf("error getting environment id: %v", err),
		}
	}

	if err := conn.CheckEnvironment(ctx, env); err != nil {
		return &model.ValidationError{
			Msg: fmt.Sprintf("error checking environment: %v", err),
		}
	}

	if _, err := s.repo.GetEnvByID(ctx, env.EnvID); err == nil {
		return &model.ConflictError{Msg: "environment already exists"}
	}

	envs := []model.Environment{*env}
	if err := s.repo.WriteEnvironments(ctx, envs); err != nil {
		return fmt.Errorf("error writing environments: %w", err)
	}

	return nil
}

func (s *EnvironmentService) GetEnvironmentForExtend(
	ctx context.Context,
	envID, token string,
) (*model.Environment, error) {
	tk, err := s.repo.GetToken(ctx, envID)
	if err != nil || tk.Token != token {
		return nil, &model.ValidationError{
			Msg: "invalid token",
		}
	}

	env, err := s.repo.GetEnvByID(ctx, envID)
	if err != nil {
		return nil, &model.NotFoundError{
			Msg: fmt.Sprintf(
				"environment not found: %v", err,
			),
		}
	}

	return env, nil
}

func (s *EnvironmentService) ExtendEnvironment(
	ctx context.Context,
	envID, period, token string,
) (*model.Environment, error) {
	tk, err := s.repo.GetToken(ctx, envID)
	if err != nil || tk.Token != token {
		return nil, &model.ValidationError{
			Msg: "invalid token",
		}
	}

	if err := utils.PeriodValidate(
		period, s.maxExtendDuration,
	); err != nil {
		return nil, &model.ValidationError{
			Msg: fmt.Sprintf("invalid period: %v", err),
		}
	}

	env, err := s.repo.GetEnvByID(ctx, envID)
	if err != nil {
		return nil, &model.NotFoundError{
			Msg: fmt.Sprintf(
				"environment not found: %v", err,
			),
		}
	}

	if err := s.repo.ExtendEnvironment(
		ctx, envID, period,
	); err != nil {
		return nil, fmt.Errorf(
			"error extending environment: %w", err,
		)
	}

	if err := s.repo.DeleteToken(ctx, env.EnvID); err != nil {
		slog.Error("error deleting token",
			slog.String("env_id", env.EnvID),
			slog.Any("error", err),
		)
	}

	env, err = s.repo.GetEnvByID(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf(
			"error fetching updated environment: %w",
			err,
		)
	}

	return env, nil
}
