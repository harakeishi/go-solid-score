package eval

// ReportJSON is the JSON-serializable view of a Report. It exists separately
// from Report so encoding can map NaN metrics — which encoding/json cannot
// represent — to JSON null, preserving the "no data" distinction that NaN
// carries in the text report.
type ReportJSON struct {
	Split        string                   `json:"split"`
	BootstrapN   int                      `json:"bootstrap_n"`
	PerPrinciple map[string]PrincipleJSON `json:"per_principle"`
}

// PrincipleJSON is one principle's metrics in JSON form. Pointer fields are
// null when the underlying metric is NaN (undefined for lack of data).
type PrincipleJSON struct {
	Precision         *float64 `json:"precision"`
	Recall            *float64 `json:"recall"`
	F1                *float64 `json:"f1"`
	F1CILow           *float64 `json:"f1_ci_low"`
	F1CIHigh          *float64 `json:"f1_ci_high"`
	RecallDenominator int      `json:"recall_denominator"`
	TP                int      `json:"tp"`
	FP                int      `json:"fp"`
	FN                int      `json:"fn"`
	TN                int      `json:"tn"`
}

// NewReportJSON projects a Report into its JSON view, turning NaN metrics into
// null pointers.
func NewReportJSON(r Report) ReportJSON {
	out := ReportJSON{
		Split:        string(r.Split),
		BootstrapN:   r.BootstrapN,
		PerPrinciple: map[string]PrincipleJSON{},
	}
	for p, pr := range r.PerPrinciple {
		m := pr.Metrics
		out.PerPrinciple[string(p)] = PrincipleJSON{
			Precision:         nilIfNaN(m.Precision),
			Recall:            nilIfNaN(m.Recall),
			F1:                nilIfNaN(m.F1),
			F1CILow:           nilIfNaN(pr.CI.F1Low),
			F1CIHigh:          nilIfNaN(pr.CI.F1High),
			RecallDenominator: m.RecallDenominator,
			TP:                m.Confusion.TP,
			FP:                m.Confusion.FP,
			FN:                m.Confusion.FN,
			TN:                m.Confusion.TN,
		}
	}
	return out
}

// nilIfNaN returns a pointer to v, or nil when v is NaN, so JSON encodes
// undefined metrics as null rather than failing on a NaN float.
func nilIfNaN(v float64) *float64 {
	if v != v { // NaN
		return nil
	}
	return &v
}
