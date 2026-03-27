package lsp

import "fmt"

// Speaker is a simple interface.
type Speaker interface {
	Speak() string
}

// Dog properly implements Speaker.
type Dog struct {
	name string
}

func (d *Dog) Speak() string {
	return fmt.Sprintf("%s says woof!", d.name)
}

// Cat properly implements Speaker.
type Cat struct {
	name string
}

func (c *Cat) Speak() string {
	return fmt.Sprintf("%s says meow!", c.name)
}
