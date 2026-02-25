package replay

import "sync"

type Manager struct {
	sessions sync.Map
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Store(s *Session) {
	m.sessions.Store(s.ID, s)
}

func (m *Manager) Get(id string) (*Session, bool) {
	v, ok := m.sessions.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*Session), true
}

func (m *Manager) Delete(id string) {
	m.sessions.Delete(id)
}
