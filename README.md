# Wire(less)

Wireless is great companion to [wire](https://github.com/google/wire) framework from google.
To get best out of both wolrds we use generated code with wire + little bit of reflection.

You can really think of wire as being container that can store dependencies.

It's only good to use it if you have the reason for it. Wire on its own is very powerful

## Example

```golang
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

func NewService(i *wireless.Injector) (*Service, func(), error) {
	c := &Config{}
	err := i.InjectAs(&c)
	if err != nil {
		return nil, nil, err
	}
	return &Service{cfg: c}, func() {}, nil
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

```
