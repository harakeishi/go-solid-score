package model

// MethodInfo represents a method attached to a struct via a receiver.
type MethodInfo struct {
	Name                 string
	ReceiverType         string
	File                 string
	LineStart            int
	LineEnd              int
	IsExported           bool
	CyclomaticComplexity int
	AccessedFields       []string
	CalledMethods        []string
	Params               []*ParamInfo
	Returns              []*ReturnInfo
	HasPanic             bool
	// HasUnconditionalPanic is true when the method panics on its straight-line
	// path (not merely inside an argument/state guard). This is the signal LSP
	// uses, since a guard panic is idiomatic fail-fast rather than a
	// substitutability violation.
	HasUnconditionalPanic bool
	IsNoop                bool
	CallsSuper            bool // calls embedded method
	TypeSwitchCount       int
	TypeAssertCount       int
	ReflectUsageCount     int
	StmtCount             int // total statements for density calculation
}

// ParamInfo represents a function/method parameter.
type ParamInfo struct {
	Name     string
	TypeName string
	IsIface  bool
	// IsEmptyIface refines IsIface: true when the type is the empty interface
	// (any / interface{}), possibly named or behind pointers/collections. An
	// empty interface abandons type information rather than abstracting
	// behavior, so OCP's interface-parameter bonus skips it.
	IsEmptyIface bool
	// IsFunc reports whether the parameter's (unwrapped) type is a function
	// type (callback/strategy) rather than a concrete collaborator.
	IsFunc bool
	// IsValue reports whether the parameter models data rather than a
	// collaborator, under the same classification as FieldInfo.IsValue (see
	// astutil.IsValueType): basic-element types, value-element struct
	// collections, and bare method-less value structs are data; pointers,
	// pointer collections, and interface elements are not.
	IsValue bool
}

// ReturnInfo represents a function/method return type.
type ReturnInfo struct {
	TypeName string
	IsIface  bool
}
