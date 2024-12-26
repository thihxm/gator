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
	"strings"
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
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("browse", middlewareLoggedIn(handlerBrowse))

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

func scrapeFeeds(s *state) error {
	feed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return err
	}

	feed, err = s.db.MarkFeedFetched(
		context.Background(),
		database.MarkFeedFetchedParams{
			ID: feed.ID,
			LastFetchedAt: sql.NullTime{
				Time:  time.Now(),
				Valid: true,
			},
			UpdatedAt: time.Now(),
		},
	)
	if err != nil {
		return err
	}

	headerTitle := fmt.Sprintf("┃  Fetching feed `%s` at %s  ┃", feed.Name, feed.Url)
	wrapper := strings.Repeat("━", len(headerTitle)-6)

	fmt.Printf("\n┏%s┓\n", wrapper)
	fmt.Println(headerTitle)
	fmt.Printf("┗%s┛\n\n", wrapper)
	data, err := fetchFeed(
		context.Background(),
		feed.Url,
	)
	if err != nil {
		return err
	}

	newPosts := 0

	for _, item := range data.Channel.Item {
		publishedAt, err := time.Parse(time.RFC1123Z, item.PubDate)
		post, err := s.db.CreatePost(
			context.Background(),
			database.CreatePostParams{
				ID:        uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Title:     item.Title,
				Url:       item.Link,
				Description: sql.NullString{
					String: item.Description,
					Valid:  item.Description != "" && len(item.Description) > 0,
				},
				PublishedAt: sql.NullTime{
					Time:  publishedAt,
					Valid: err == nil,
				},
				FeedID: feed.ID,
			},
		)
		if err != nil {
			if !strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				fmt.Printf("Error creating post for: %s\n", item.Title)
				fmt.Printf("Error: %v\n", err)
			}
			continue
		}

		newPosts++
		fmt.Printf("Title: %s\n", post.Title)
	}

	if newPosts == 0 {
		fmt.Println("No new posts found")
	} else {
		fmt.Printf("\nFound %d new post(s)\n", newPosts)
	}
	fmt.Printf("\n━%s━\n\n", wrapper)

	return nil
}
