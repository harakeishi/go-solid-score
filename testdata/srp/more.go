package srp

import (
	"fmt"
	"os"
	"strings"
)

// UserManager mixes three unrelated concerns — user state, email delivery and
// audit logging — in one type. Its method clusters touch disjoint field
// groups.
// solid:want SRP=violation reason="mixed responsibilities: user CRUD, SMTP delivery and audit logging cluster over disjoint fields (users vs smtpHost/smtpPort vs auditPath)"
type UserManager struct {
	users     map[string]string
	smtpHost  string
	smtpPort  int
	auditPath string
}

func (u *UserManager) AddUser(id, name string)   { u.users[id] = name }
func (u *UserManager) RemoveUser(id string)      { delete(u.users, id) }
func (u *UserManager) GetUser(id string) string  { return u.users[id] }
func (u *UserManager) SendWelcomeMail(to string) { fmt.Println("smtp", u.smtpHost, u.smtpPort, to) }
func (u *UserManager) SendResetMail(to string)   { fmt.Println("smtp", u.smtpHost, u.smtpPort, to) }
func (u *UserManager) WriteAudit(entry string)   { _ = os.WriteFile(u.auditPath, []byte(entry), 0644) }
func (u *UserManager) RotateAudit(suffix string) { _ = os.Rename(u.auditPath, u.auditPath+suffix) }

// TinyMixed is a small type (four methods) that still has two disconnected
// responsibilities: counting and labelling never share a field. Low cohesion
// is not exclusive to god-class-sized types.
// solid:want SRP=violation reason="small but incohesive: counter methods (n) and label methods (label) form two disjoint clusters sharing no field"
type TinyMixed struct {
	n     int
	label string
}

func (t *TinyMixed) Inc() int            { t.n++; return t.n }
func (t *TinyMixed) Reset()              { t.n = 0 }
func (t *TinyMixed) SetLabel(s string)   { t.label = s }
func (t *TinyMixed) FormatLabel() string { return "[" + t.label + "]" }

// ConfigBlob aggregates parsing, validation, persistence and diffing of a
// config document; the persistence pair and parse/validate pair operate on
// different halves of the state.
// solid:want SRP=violation reason="parse/validate operate on raw, save/load on path, diff on snapshots — three method clusters over disjoint field groups"
type ConfigBlob struct {
	raw      []byte
	path     string
	snapshot map[string]string
	previous map[string]string
}

func (c *ConfigBlob) Parse() error {
	if len(c.raw) == 0 {
		return fmt.Errorf("empty")
	}
	return nil
}

func (c *ConfigBlob) Validate() bool { return len(c.raw) > 0 }

func (c *ConfigBlob) SaveFile() error { return os.WriteFile(c.path, nil, 0644) }

func (c *ConfigBlob) LoadFile() error {
	_, err := os.ReadFile(c.path)
	return err
}

func (c *ConfigBlob) Diff() []string {
	var out []string
	for k, v := range c.snapshot {
		if c.previous[k] != v {
			out = append(out, k)
		}
	}
	return out
}

func (c *ConfigBlob) Snapshot() { c.previous = c.snapshot }

// DeepMatcher has one responsibility (matching) but implements it with a
// single deeply nested method. High cognitive complexity alone must lower the
// SRP score moderately, yet a cohesive single-purpose type must NOT cross the
// violation threshold — this is the boundary case for the cognitive rule.
// solid:want SRP=ok reason="single cohesive responsibility (pattern matching over pattern/flags); one hard-to-read method is a readability smell, not a second responsibility"
type DeepMatcher struct {
	pattern string
	flags   int
}

func (d *DeepMatcher) Match(lines []string) []string {
	var out []string
	for _, line := range lines {
		if d.flags > 0 {
			if strings.Contains(line, d.pattern) {
				if len(line) > 3 {
					if line[0] != '#' {
						out = append(out, line)
					}
				}
			}
		} else {
			if strings.HasPrefix(line, d.pattern) {
				out = append(out, line)
			}
		}
	}
	return out
}

func (d *DeepMatcher) SetFlags(f int) { d.flags = f }

// Wallet is a cohesive value-holder: every method reads or writes the single
// balance field.
// solid:want SRP=ok reason="all methods operate on balance — one responsibility (LSCC high)" split=train
type Wallet struct {
	balance int64
}

func (w *Wallet) Deposit(v int64)  { w.balance += v }
func (w *Wallet) Withdraw(v int64) { w.balance -= v }
func (w *Wallet) Balance() int64   { return w.balance }
func (w *Wallet) Empty() bool      { return w.balance == 0 }

// RingBuffer is bigger than Wallet but still fully cohesive: head, tail and
// buf participate in every operation. Size alone must not flag it.
// solid:want SRP=ok reason="classic data structure: every method coordinates the same buf/head/tail state — cohesive despite method count"
type RingBuffer struct {
	buf  []byte
	head int
	tail int
}

func (r *RingBuffer) Push(b byte) {
	r.buf[r.tail] = b
	r.tail = (r.tail + 1) % len(r.buf)
}

func (r *RingBuffer) Pop() byte {
	b := r.buf[r.head]
	r.head = (r.head + 1) % len(r.buf)
	return b
}

func (r *RingBuffer) Len() int {
	if r.tail >= r.head {
		return r.tail - r.head
	}
	return len(r.buf) - r.head + r.tail
}

func (r *RingBuffer) Cap() int   { return len(r.buf) }
func (r *RingBuffer) Full() bool { return (r.tail+1)%len(r.buf) == r.head }
func (r *RingBuffer) Reset()     { r.head, r.tail = 0, 0 }
