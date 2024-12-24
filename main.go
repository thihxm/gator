package main

import (
	"fmt"
	"os"

	"github.com/thihxm/gator/internal/config"
)

type state struct {
	config *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	cmds map[string]func(*state, command) error
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.cmds[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	handler, ok := c.cmds[cmd.name]
	if !ok {
		return fmt.Errorf("command not found")
	}

	return handler(s, cmd)
}

func main() {
	cmds := commands{
		cmds: make(map[string]func(*state, command) error),
	}
	cmds.register("login", handlerLogin)

	loadedConfig, err := config.Read()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
	s := &state{
		config: &loadedConfig,
	}

	args := os.Args

	if len(args) < 2 {
		fmt.Println("Usage: gator <command> <args>")
		os.Exit(1)
		return
	}

	cmd := command{name: args[1], args: args[2:]}
	if err := cmds.run(s, cmd); err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("the login command requires a username")
	}

	err := s.config.SetUser(cmd.args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Logged in as %s\n", cmd.args[0])

	return nil
}
