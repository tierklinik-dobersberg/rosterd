package server

type StatusWrapper struct {
	Status int
	Value  interface{}
}

func withStatus(code int, value interface{}) (*StatusWrapper, error) {
	return &StatusWrapper{
		Status: code,
		Value:  value,
	}, nil
}
