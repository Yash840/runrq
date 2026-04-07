package domain

type JhRegistry interface {
	GetHandler(Type string) (JobProcessor, error)
	AddHandler(Type string, Handler JobProcessor)
}
