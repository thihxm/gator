# Gator

Gator is a simple RSS feed aggregator written in Go. It allows you to aggregate posts from multiple RSS feeds and browse them in a single place.
It was created as a learning project to get familiar with Go and PostgreSQL for the Boot.dev course.

# Requirements:

-   PostgreSQL
-   Go 1.23.2 or higher

<!-- Explain how the user can install `gator` using go install -->

# Installation:

```bash
go install github.com/username/repo
```

# Usage:

First, you need to create a configuration file `.gatorconfig.json` in the root of your user directory. The configuration file should look like this:

```json
{
    // The connection string to the database
    "db_url": "postgres://postgres:postgres@localhost:5432/gator?sslmode=disable"
}
```

Then you can run the following command to start the application:

```bash
gator <command> <args>
```

Example commands:

```bash
# Register and login as an user
gator register <username>

# Login as an user
gator login <username>

# List all users
gator users

# Aggregate posts from all RSS feeds every x period of time
# The time period should be in the format of 1s, 1m, 1h, 1d
gator agg <timeperiod>

# Add a new RSS feed
gator addfeed <name> <url>

# List all RSS feeds
gator feeds

# Follow a feed
gator follow <url>

# Unfollow a feed
gator unfollow <url>

# List all followed feeds
gator following

# Browse posts
# The limit is optional and defaults to 2
gator browse <limit>
```
