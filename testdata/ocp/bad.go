package ocp

import "fmt"

// Router uses type switches extensively instead of polymorphism.
type Router struct {
	routes []interface{}
}

func (r *Router) Route(msg interface{}) {
	switch v := msg.(type) {
	case string:
		fmt.Println("string:", v)
	case int:
		fmt.Println("int:", v)
	case float64:
		fmt.Println("float:", v)
	case []byte:
		fmt.Println("bytes:", v)
	}
}

func (r *Router) Validate(msg interface{}) bool {
	switch msg.(type) {
	case string:
		return true
	case int:
		return true
	default:
		return false
	}
}

func (r *Router) TypeName(msg interface{}) string {
	if _, ok := msg.(string); ok {
		return "string"
	}
	if _, ok := msg.(int); ok {
		return "int"
	}
	return "unknown"
}
