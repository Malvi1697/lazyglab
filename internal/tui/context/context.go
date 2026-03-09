package context

// Kind represents the type of context in the navigation stack.
type Kind int

const (
	KindSide  Kind = iota // Side panel list view
	KindMain              // Main detail view
	KindPopup             // Overlay (help, confirmation, search)
)

// Context represents a navigable view in the context stack.
type Context struct {
	Kind Kind
	Name string
}
