package dip

// Repository is an interface - depends on abstraction.
type Repository interface {
	Save(id string, data []byte) error
	Load(id string) ([]byte, error)
}

// Logger is an interface for logging.
type Logger interface {
	Log(msg string)
}

// Service depends on interfaces, not concrete types.
type Service struct {
	repo   Repository
	logger Logger
}

func NewService(repo Repository, logger Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

func (s *Service) Process(id string, data []byte) error {
	s.logger.Log("processing " + id)
	return s.repo.Save(id, data)
}
