# Reddit-Discord-Bot

Simple webhook based bot delivering Reddit posts into your Discord Server

The bot does not require any permissions and can't do anything in your Discord. With that it's safe to use!

<details>
<summary>Table of Contents</summary>

- [Usage](#usage)
	- [Add Subreddit](#add-subreddit)
	- [Remove Subreddit](#remove-subreddit)
	- [List Subreddits](#list-subreddits)
- [Public Bot](#public-bot)
- [Self-hosted](#self-hosted)
	- [Binary](#binary)
	- [Docker-Compose](#docker-compose)
- [Help](#help)
- [License](#license)
- [Contributing](#contributing)
- [Contact](#contact)

</details>

## Usage

### Add Subreddit

To add a new subreddit run

```bash
/subreddit add <subreddit-name>
```

and click the returned link

select the server & channel in the discord popup & hit okay that's all!

### Remove Subreddit

To remove a subreddit subscriptions run

```bash
/subreddit remove <subreddit-name>
```

and remove the bot under `Server Settings > Integrations` or just delete the webhook under `Channel Settings > Integrations > Webhooks`

### List Subreddits

To list all subreddit subscriptions run

```bash
/subreddit list
```

## Public Bot

Invite the bot [here](https://discord.com/oauth2/authorize?client_id=846396249241288796&scope=applications.commands)

## Self-hosted

Reddit-Discord-Bot is now super easy to self-host. You can either use the docker image or build a binary yourself.

Before you can run the bot you need to create a config file. You can find an example [here](/config.example.yml)

You also need a discord bot token, which you can get from [here](https://discord.com/developers/applications).

Lastly you need to create a reddit app which can be done [here](https://www.reddit.com/prefs/apps/).

The bot requires a database, which can either be `SQLite` or `PostgreSQL`. Just select the one you want to use in the config file.

### Binary

Prerequisites:

- [Go](https://golang.org/doc/install)
- [Git](https://git-scm.com/downloads)
- [PostgreSQL](https://www.postgresql.org/download/) (optional)

```bash
$ git clone git@github.com:topi314/Reddit-Discord-Bot.git
$ cd Reddit-Discord-Bot
$ go build -o reddit-discord-bot .
```

You can now run the bot with

```bash
$ ./reddit-discord-bot -config config.yml
```

### Docker-Compose

```bash
version: "3.8"

services:
  reddit-bot:
    image: ghcr.io/topi314/reddit-discord-bot:master
    container_name: reddit-bot
    restart: unless-stopped
    volumes:
	  - ./config.yml:/var/lib/reddit-discord-bot/config.yml
	  - ./database.db:/var/lib/reddit-discord-bot/database.db
```

You can now run the bot with

```bash
$ docker-compose up -d
```

# Help

If you encounter any problems feel free to open an issue or reach out to me(`toπ#3141`) via discord [here](https://discord.gg/RKM92xXu4Y)

# License

Reddit-Discord-Bot is licensed under the [Apache License 2.0](/LICENSE).

# Contributing

Contributions are always welcome! Just open a pull request or discussion and I will take a look at it.

## Contact

- [Discord](https://discord.gg/sD3ABd5)
- [Twitter](https://twitter.com/topi314)
- [Email](mailto:git@topi.wtf)