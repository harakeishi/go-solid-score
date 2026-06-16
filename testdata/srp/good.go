package srp

// TaxCalculator is a cohesive struct with all methods accessing the same fields.
// solid:want SRP=ok reason="all methods access the same rate/discount fields — single cohesive responsibility (LCOM4=1)"
type TaxCalculator struct {
	rate     float64
	discount float64
}

func NewTaxCalculator(rate, discount float64) *TaxCalculator {
	return &TaxCalculator{rate: rate, discount: discount}
}

func (t *TaxCalculator) Calculate(amount float64) float64 {
	return amount * t.rate
}

func (t *TaxCalculator) CalculateWithDiscount(amount float64) float64 {
	tax := amount * t.rate
	return tax - (tax * t.discount)
}

func (t *TaxCalculator) EffectiveRate() float64 {
	return t.rate * (1 - t.discount)
}

// ParseError is a cohesive error type. Its Error method uses the receiver's
// field, while Is is a standard errors.Is convention method that only inspects
// its argument and touches no field. The stateless Is method must not fragment
// LCOM4 and drag SRP down — the type has a single responsibility.
// solid:want SRP=ok reason="cohesive error type; the stateless errors.Is convention method is not a second responsibility"
type ParseError struct {
	msg string
}

func (e ParseError) Error() string {
	return "parse error: " + e.msg
}

func (e ParseError) Is(target error) bool {
	_, ok := target.(ParseError)
	return ok
}
