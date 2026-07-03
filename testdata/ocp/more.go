package ocp

import (
	"fmt"
	"reflect"
)

// Exporter dispatches on concrete types with a chain of comma-ok assertions —
// the assertion-chain flavour of the same closed-for-extension defect as
// Router's type switches.
// solid:want OCP=violation reason="assertion chain over concrete types (string/int/float64/bool): every new exportable type requires editing Export"
type Exporter struct {
	sink []string
}

func (e *Exporter) Export(v interface{}) {
	if s, ok := v.(string); ok {
		e.sink = append(e.sink, s)
		return
	}
	if i, ok := v.(int); ok {
		e.sink = append(e.sink, fmt.Sprint(i))
		return
	}
	if f, ok := v.(float64); ok {
		e.sink = append(e.sink, fmt.Sprint(f))
		return
	}
	if b, ok := v.(bool); ok {
		e.sink = append(e.sink, fmt.Sprint(b))
		return
	}
}

func (e *Exporter) ExportAll(vs []interface{}) {
	for _, v := range vs {
		switch t := v.(type) {
		case string:
			e.sink = append(e.sink, t)
		case int:
			e.sink = append(e.sink, fmt.Sprint(t))
		}
	}
}

func (e *Exporter) Kind(v interface{}) string {
	if _, ok := v.(string); ok {
		return "string"
	}
	if _, ok := v.(int); ok {
		return "int"
	}
	return "other"
}

// ReflectMapper drives its dispatch through reflection: kind switches and
// reflect calls stand in for polymorphism, so supporting a new kind means
// editing this type.
// solid:want OCP=violation reason="reflect-based dispatch: switches on reflect.Kind and concrete types — new kinds require modifying Map/Describe"
type ReflectMapper struct {
	out map[string]string
}

func (r *ReflectMapper) Map(v interface{}) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		r.out["s"] = rv.String()
	case reflect.Int, reflect.Int64:
		r.out["i"] = fmt.Sprint(rv.Int())
	case reflect.Bool:
		r.out["b"] = fmt.Sprint(rv.Bool())
	}
}

func (r *ReflectMapper) Describe(v interface{}) string {
	t := reflect.TypeOf(v)
	switch v.(type) {
	case string:
		return "string:" + t.Name()
	case int:
		return "int:" + t.Name()
	default:
		return t.Kind().String()
	}
}

// KindRegistry branches on an internal kind enum instead of concrete Go
// types. This is the classic Meyer/Martin OCP violation shape even though no
// type switch appears — a boundary case that value-switch-blind detection
// misses (violation that can look ok to the tool).
// solid:want OCP=violation reason="enum-style switch on a kind field: adding a new kind means editing Handle and Describe, not adding a new implementation"
type KindRegistry struct {
	kinds []int
}

func (k *KindRegistry) Handle(kind int, payload string) string {
	switch kind {
	case 0:
		return "text:" + payload
	case 1:
		return "json:" + payload
	case 2:
		return "yaml:" + payload
	default:
		return "unknown"
	}
}

func (k *KindRegistry) Describe(kind int) string {
	switch kind {
	case 0:
		return "text"
	case 1:
		return "json"
	default:
		return "other"
	}
}

// Notifier fans out to an abstraction: new channels are added by implementing
// Channel, never by editing Notifier.
// solid:want OCP=ok reason="extension happens by adding Channel implementations; Notifier itself never changes"
type Notifier struct {
	channels []Channel
}

// Channel abstracts a delivery mechanism.
type Channel interface {
	Deliver(msg string) error
}

func (n *Notifier) Notify(msg string) {
	for _, c := range n.channels {
		_ = c.Deliver(msg)
	}
}

func (n *Notifier) Register(c Channel) {
	n.channels = append(n.channels, c)
}

// StatsCollector does arithmetic over plain data — lots of branching but zero
// type inspection. Ordinary value branching must not be mistaken for an OCP
// smell (ok that could look like a violation to density-style heuristics).
// solid:want OCP=ok reason="branches on values, not types; no type switches, assertions or reflection anywhere" split=train
type StatsCollector struct {
	min, max, sum, n int
}

func (s *StatsCollector) Observe(v int) {
	if s.n == 0 || v < s.min {
		s.min = v
	}
	if s.n == 0 || v > s.max {
		s.max = v
	}
	s.sum += v
	s.n++
}

func (s *StatsCollector) Mean() int {
	if s.n == 0 {
		return 0
	}
	return s.sum / s.n
}
