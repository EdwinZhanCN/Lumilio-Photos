package runtimeconfig

import (
	"context"

	"desktop/internal/control/dto"
)

type DesiredStateStore struct {
	Path  string
	Lumen bool
}

func NewDesiredStateStore(path string) *DesiredStateStore {
	return &DesiredStateStore{Path: path}
}

func NewLumenDesiredStateStore(path string) *DesiredStateStore {
	return &DesiredStateStore{Path: path, Lumen: true}
}

func (s *DesiredStateStore) Load(_ context.Context) (dto.DesiredState, error) {
	settings, err := LoadSettings(s.Path)
	if err != nil {
		return "", err
	}
	if s.Lumen {
		return settings.LumenDesiredState, nil
	}
	return settings.RuntimeDesiredState, nil
}

func (s *DesiredStateStore) Save(_ context.Context, desired dto.DesiredState) error {
	settings, err := LoadSettings(s.Path)
	if err != nil {
		return err
	}
	if s.Lumen {
		settings.LumenDesiredState = desired
	} else {
		settings.RuntimeDesiredState = desired
	}
	return SaveSettings(s.Path, settings)
}
