package dip

import (
	"database/sql"
	"log"
)

// BadService depends on concrete types directly.
type BadService struct {
	db     *sql.DB
	logger *log.Logger
}

func NewBadService() *BadService {
	db, _ := sql.Open("mysql", "dsn")
	return &BadService{
		db:     db,
		logger: log.Default(),
	}
}

func (s *BadService) Save(data string) error {
	s.logger.Println("saving", data)
	_, err := s.db.Exec("INSERT INTO data VALUES (?)", data)
	return err
}
