package Storage

type Storage interface {
	Has(hash string) bool
	Mark(hash string)
	InitState(hash string, neighbors []string) error
	UpdateState(hash string, from string) error
	GetStates(hash string) (map[string]bool, error)
	GetState(hash, nodeHash string) bool
	MarkSent(hash, nodeHash string)
	DeleteState(hash string)
}
