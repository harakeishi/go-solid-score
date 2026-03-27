package srp

import (
	"fmt"
	"os"
)

// GodStruct has multiple disconnected responsibilities.
type GodStruct struct {
	name    string
	email   string
	db      string
	logFile string
	cache   map[string]string
}

func (g *GodStruct) GetName() string {
	return g.name
}

func (g *GodStruct) SetEmail(email string) {
	g.email = email
}

func (g *GodStruct) SaveToDatabase() {
	_ = g.db
	fmt.Println("saving to", g.db)
}

func (g *GodStruct) LoadFromDatabase() {
	_ = g.db
	fmt.Println("loading from", g.db)
}

func (g *GodStruct) WriteLog(msg string) {
	_ = g.logFile
	_ = os.WriteFile(g.logFile, []byte(msg), 0644)
}

func (g *GodStruct) ReadLog() string {
	data, _ := os.ReadFile(g.logFile)
	return string(data)
}

func (g *GodStruct) CacheGet(key string) string {
	return g.cache[key]
}

func (g *GodStruct) CacheSet(key, value string) {
	g.cache[key] = value
}
