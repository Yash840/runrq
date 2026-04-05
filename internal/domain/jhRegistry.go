package domain

type JhRegistry interface {
	GetHandler(Type string) (JobHandler, error)
	AddHandler(Type string, Handler JobHandler)
}

