package dip

import "database/sql"

// Mailer is a concrete collaborator with behavior (not a data record).
type Mailer struct {
	host string
}

func (m *Mailer) Send(to, body string) error { return nil }

// Enroller receives its collaborator through the constructor — but the
// parameter is the concrete *Mailer, so the injection inverts nothing: the
// high-level policy still names the low-level detail.
// solid:want DIP=violation reason="constructor injection of a concrete *Mailer — injection without abstraction still couples policy to detail"
type Enroller struct {
	mailer *Mailer
}

func NewEnroller(m *Mailer) *Enroller {
	return &Enroller{mailer: m}
}

func (e *Enroller) Enroll(user string) error {
	return e.mailer.Send(user, "welcome")
}

// Archiver is one interface short of inverted: the store is an abstraction but
// the mailer and database remain concrete, so most of its dependency weight
// still points at details.
// solid:want DIP=violation reason="mixed dependencies: one abstract (Repository) against concrete *Mailer and *sql.Tx — majority of dependency weight is concrete"
type Archiver struct {
	repo   Repository
	mailer *Mailer
	tx     *sql.Tx
}

func NewArchiver(repo Repository, m *Mailer, tx *sql.Tx) *Archiver {
	return &Archiver{repo: repo, mailer: m, tx: tx}
}

func (a *Archiver) Archive(id string, data []byte) error {
	if err := a.repo.Save(id, data); err != nil {
		return err
	}
	return a.mailer.Send("admin", "archived "+id)
}

// Sender abstracts message delivery for the ok cases below.
type Sender interface {
	Send(to, body string) error
}

// Greeter is Enroller done right: the same shape, but the constructor takes
// the Sender abstraction, so any delivery mechanism substitutes.
// solid:want DIP=ok reason="depends on the Sender abstraction via constructor injection; no concrete collaborator named"
type Greeter struct {
	sender Sender
}

func NewGreeter(s Sender) *Greeter {
	return &Greeter{sender: s}
}

func (g *Greeter) Greet(user string) error {
	return g.sender.Send(user, "hello")
}

// Retrier owns no collaborators at all: its dependencies are a callback and
// plain configuration values, which DIP has no business penalizing.
// solid:want DIP=ok reason="depends on a func value and scalar config only; there is no concrete collaborator to invert" split=train
type Retrier struct {
	attempt func() error
	max     int
}

func NewRetrier(attempt func() error, max int) *Retrier {
	return &Retrier{attempt: attempt, max: max}
}

func (r *Retrier) Run() error {
	var err error
	for i := 0; i < r.max; i++ {
		if err = r.attempt(); err == nil {
			return nil
		}
	}
	return err
}
