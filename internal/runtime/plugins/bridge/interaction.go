package bridge

type Interaction interface {
	Defer(ephemeral bool) error
}
