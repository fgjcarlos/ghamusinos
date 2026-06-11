package handlers

import (
	"context"
	"errors"
)

// errPingFailed es el error centinela que usan los tests para simular un ping fallido.
var errPingFailed = errors.New("ping fallido (simulado)")

// fakePinger implementa DBPinger para tests sin base de datos real.
type fakePinger struct {
	err error
}

func (f *fakePinger) Ping(_ context.Context) error {
	return f.err
}
