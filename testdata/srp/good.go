package srp

// TaxCalculator is a cohesive struct with all methods accessing the same fields.
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
