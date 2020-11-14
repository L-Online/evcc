package cmd

import (
	"fmt"
	"net"
)

type TaskHandler interface {
	Test(ip net.IP) bool
}

type TaskHandlerRegistry map[string]func(map[string]interface{}) (TaskHandler, error)

func (r TaskHandlerRegistry) Add(name string, factory func(map[string]interface{}) (TaskHandler, error)) {
	if _, exists := r[name]; exists {
		panic(fmt.Sprintf("cannot register duplicate charger type: %s", name))
	}
	r[name] = factory
}

func (r TaskHandlerRegistry) Get(name string) (func(map[string]interface{}) (TaskHandler, error), error) {
	factory, exists := r[name]
	if !exists {
		return nil, fmt.Errorf("charger type not registered: %s", name)
	}
	return factory, nil
}
