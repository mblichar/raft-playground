package timer

type TimeoutFactory interface {
	Timeout(kind string, milliseconds uint) Timeout
}

type Timeout interface {
	Done() <-chan struct{}
}
