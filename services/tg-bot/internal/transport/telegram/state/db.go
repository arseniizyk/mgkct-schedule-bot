package state

import (
	"context"
)

type Store interface {
	GetState(ctx context.Context, chatID int64) (string, error)
	SetState(ctx context.Context, chatID int64, state string) error
}

type DBManager struct {
	store Store
}

func NewDB(store Store) *DBManager {
	return &DBManager{store: store}
}

func (m *DBManager) Clear(chatID int64) error {
	return m.store.SetState(context.Background(), chatID, "")
}

func (m *DBManager) Get(chatID int64) (State, bool) {
	s, err := m.store.GetState(context.Background(), chatID)
	if err != nil || s == "" {
		return "", false
	}
	return State(s), true
}

func (m *DBManager) Set(chatID int64, st State) error {
	return m.store.SetState(context.Background(), chatID, string(st))
}
