package dip

// Tree is a recursive aggregate whose fields are value data, callbacks, and
// self-references rather than injected service collaborators. DIP should not
// penalize it: none of these fields are concrete dependencies that ought to
// be inverted behind an interface.
type Tree struct {
	Name     string            // value
	tags     map[string]string // value data container
	children []*Tree           // self-reference (tree structure)
	parent   *Tree             // self-reference
	OnVisit  func(n *Tree)     // callback / strategy
	handlers []Logger          // slice of interface -> abstraction dependency
}

func NewTree(handlers []Logger) *Tree {
	return &Tree{handlers: handlers}
}

// Printer owns no collaborators: its only non-value "dependency" is the
// concrete *Tree supplied to a method at call time. DIP is not structurally
// applicable here, so the type must not be penalized toward zero.
type Printer struct {
	Indent int
}

func (p *Printer) Print(t *Tree) string {
	return t.Name
}

func (t *Tree) Visit() {
	if t.OnVisit != nil {
		t.OnVisit(t)
	}
	for _, h := range t.handlers {
		h.Log("visiting " + t.Name)
	}
	for _, c := range t.children {
		c.Visit()
	}
}
