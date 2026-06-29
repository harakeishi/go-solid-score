package dip

import "sync"

// Message is a behavior-less data record (a DTO). It is held, not invoked.
type Message struct {
	ID   string
	Text string
}

// TS is a pure value formatter, not behavior a collaborator exposes.
func (m Message) TS() string { return m.ID }

// DataContainer holds a slice of plain data records ([]Message). The only field
// surviving the DIP dependency filter is the value-element slice, which is
// storage the struct owns — not an inverted concrete collaborator it calls into.
// DIP must not penalize it: holding your own data records is not an uninverted
// dependency on a low-level detail in the Martin sense, it is just storage.
// solid:want DIP=ok reason="holds a slice of plain data records ([]Message); a value-element data container is storage, not an inverted concrete collaborator dependency"
type DataContainer struct {
	mu   sync.Mutex // whitelisted -> skipped
	msgs []Message  // value-element data container -> must be skipped
	max  int        // basic value -> skipped
}

func (c *DataContainer) Add(id, text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, Message{ID: id, Text: text})
}

func (c *DataContainer) List() []Message { return c.msgs }

// Worker is a concrete struct collaborator. Like facade.go's `stage`, it has no
// behavior methods and only exposes a field — the pointer in the owning slice,
// not the method set, is what marks it a collaborator.
type Worker struct{ id int }

// CollaboratorOwner owns a *pointer* collection of a concrete struct collaborator
// (`[]*Worker`), mirroring facade.go's Pipeline (`[]*stage`). This is a genuine
// concrete dependency not held behind an interface and must REMAIN a DIP
// violation — the regression guard proving the data-container fix does not
// reintroduce a false negative on pointer-element collaborator collections.
// solid:want DIP=violation reason="owns []*Worker, a pointer collection of a concrete struct collaborator not held behind an interface — a genuine concrete dependency that must remain flagged"
type CollaboratorOwner struct {
	workers []*Worker
}

func (o *CollaboratorOwner) Run() {
	for _, w := range o.workers {
		_ = w.id
	}
}
