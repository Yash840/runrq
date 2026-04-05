package domain

import "errors"

type DefaultRegistry struct{
	Registry map[string]JobHandler
}

func (dr DefaultRegistry) GetHandler(Type string) (JobHandler, error) {
	handler, ok := dr.Registry[Type]

	if ok == false {
		return nil, errors.New("handler for given type is not available")
	}

	return handler, nil
}

func (dr DefaultRegistry) AddHandler(Type string, Handler JobHandler) {
	dr.Registry[Type] = Handler
}