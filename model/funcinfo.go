package model

// FuncInfo represents a package-level function (no receiver).
type FuncInfo struct {
	Name                 string
	File                 string
	LineStart            int
	LineEnd              int
	IsExported           bool
	CyclomaticComplexity int
	Params               []*ParamInfo
	Returns              []*ReturnInfo
	TypeSwitchCount      int
	TypeAssertCount      int
	ReflectUsageCount    int
	StmtCount            int
}
