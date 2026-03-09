package context

// Manager maintains a LIFO stack of navigation contexts.
type Manager struct {
	stack []Context
}

// NewManager creates a new context manager with a root context.
func NewManager() *Manager {
	return &Manager{
		stack: []Context{
			{Kind: KindSide, Name: "root"},
		},
	}
}

// Push adds a context to the stack.
func (m *Manager) Push(ctx Context) {
	m.stack = append(m.stack, ctx)
}

// Pop removes the top context from the stack.
// Returns false if only the root context remains.
func (m *Manager) Pop() bool {
	if len(m.stack) <= 1 {
		return false
	}
	m.stack = m.stack[:len(m.stack)-1]
	return true
}

// Current returns the top context on the stack.
func (m *Manager) Current() Context {
	return m.stack[len(m.stack)-1]
}

// Depth returns the current stack depth.
func (m *Manager) Depth() int {
	return len(m.stack)
}
