package timer

type Timer interface {
	Timeout(kind string, milliseconds uint) Timeout
	Cancel()
}

type Timeout interface {
	Done() <-chan struct{}
	Cancel()
}
