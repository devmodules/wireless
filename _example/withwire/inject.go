//go:build wireinject
// +build wireinject

package main

import (
	"github.com/devmodules/wireless"
	"github.com/google/wire"
)

// New creates a new service implementation.
func NewService(i *wireless.Injector) (*Service, func(), error) {
	wire.Build(
		NewLogger,
		ConfigInject,
		wire.Struct(new(Service), "*"),
	)
	return new(Service), func() {}, nil
}

func ConfigInject(i *wireless.Injector) (*Config, error) {
	var c *Config = &Config{}
	err := i.InjectAs(&c)
	return c, err
}
