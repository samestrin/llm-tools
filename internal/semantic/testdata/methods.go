package testdata

// Calculator performs arithmetic operations
type Calculator struct {
	Value int
}

// Add adds n to the calculator's value
func (c *Calculator) Add(n int) {
	c.Value += n
}

// Reset resets the calculator to zero
func (c Calculator) Reset() Calculator {
	c.Value = 0
	return c
}
