package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/thihxm/gator/internal/config"
	"github.com/thihxm/gator/internal/database"
)

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type state struct {
	db  *database.Queries
	cfg *config.Config
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
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", handlerAddFeed)

	loadedConfig, err := config.Read()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}

	db, err := sql.Open("postgres", loadedConfig.DB_URL)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}

	dbQueries := database.New(db)
	defer db.Close()

	s := &state{
		cfg: &loadedConfig,
		db:  dbQueries,
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

	_, err := s.db.GetUser(
		context.Background(),
		cmd.args[0],
	)
	if err != nil {
		return err
	}

	err = s.cfg.SetUser(cmd.args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Logged in as %s\n", cmd.args[0])

	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("the login command requires a username")
	}

	user, err := s.db.CreateUser(
		context.Background(),
		database.CreateUserParams{
			ID:        uuid.New(),
			Name:      cmd.args[0],
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	)

	if err != nil {
		return err
	}

	err = s.cfg.SetUser(user.Name)
	if err != nil {
		return err
	}

	fmt.Println("User registered successfully")
	fmt.Printf("New user: %v\n", user)

	return nil
}

func handlerReset(s *state, cmd command) error {
	err := s.db.ResetUsers(context.Background())
	if err != nil {
		return err
	}

	return nil
}

func handlerUsers(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return err
	}

	for _, user := range users {
		if user.Name == *s.cfg.CurrentUserName {
			fmt.Printf("* %s (current)\n", user.Name)
		} else {
			fmt.Printf("* %s\n", user.Name)
		}
	}

	return nil
}

func handlerAgg(s *state, cmd command) error {
	feed, err := fetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		return err
	}

	fmt.Println(feed)

	return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "gator")

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var feed RSSFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}

	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)

	for i := 0; i < len(feed.Channel.Item); i++ {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
	}

	return &feed, nil
}

func handlerAddFeed(s *state, cmd command) error {
	if len(cmd.args) <= 1 {
		return fmt.Errorf("the add command requires a feed URL and a name")
	}

	name := cmd.args[0]
	url := cmd.args[1]

	currentUser, err := s.db.GetUser(
		context.Background(),
		*s.cfg.CurrentUserName,
	)
	if err != nil {
		return err
	}

	feed, err := s.db.CreateFeed(
		context.Background(),
		database.CreateFeedParams{
			ID:        uuid.New(),
			Name:      name,
			Url:       url,
			UserID:    currentUser.ID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	)
	if err != nil {
		return err
	}

	fmt.Println(feed)

	return nil
}
