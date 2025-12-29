package testdata

// Reader defines a read interface
type Reader interface {
	// Read reads data into p
	Read(p []byte) (n int, err error)
}

// Writer defines a write interface
type Writer interface {
	// Write writes data from p
	Write(p []byte) (n int, err error)
}

// ReadWriter combines Reader and Writer
type ReadWriter interface {
	Reader
	Writer
}
