package main

import (
	"github.com/routercore/wireless"
)

// Dependency as an example of dependency.
type Logger struct{}

func NewLogger() *Logger { return &Logger{} }

func (*Logger) Log(s string) { print(s) }

type Config struct{ Addr string }

// Service is an example of service you want to run.
type Service struct {
	log *Logger
	cfg *Config
}

func (s *Service) Run() {
	s.log.Log("running service on address: " + s.cfg.Addr)
}

func main() {
	cfg := &Config{
		Addr: "localhost",
	}
	i := wireless.New()

	i.Provide(
		wireless.Value(cfg),
		wireless.Func(NewService),
	)
	if err := i.Resolve(); err != nil {
		panic(err)
	}
	defer i.Clean()

	var s *Service
	if err := i.InjectAs(&s); err != nil {
		panic(err)
	}

	s.Run()
}
