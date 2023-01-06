package timer

type TimeoutFactory interface {
	Timeout(kind string, milliseconds int) Timeout
}

type Timeout interface {
	Done() <-chan struct{}
}
