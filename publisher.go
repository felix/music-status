package mstatus

type Publisher interface {
	Publish(Status) error
}
