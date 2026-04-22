package domain

type JobHandler interface {
	Process(job Job)
}

type JhRegistry interface {
	GetHandler(Type string) (JobHandler, error)
	AddHandler(Type string, Handler JobHandler)
}
