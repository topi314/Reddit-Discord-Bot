# Reddit-Discord-Bot

Simple webhook based bot delivering Reddit posts into your Discord Server

The bot does not require any permissions and can't do anything in your Discord. With that it's safe to use!

# Setup

Invite the bot [here](https://discord.com/oauth2/authorize?client_id=846396249241288796&scope=applications.commands)

## Add Subreddit

To add a new subreddit run

```bash
/subreddit add <subreddit-name>
```

and click the returned link

select the server & channel in the discord popup & hit okay that's all!

## Remove Subreddit

To remove a subreddit subscriptions run

```bash
/subreddit remove <subreddit-name>
```

and remove the bot under `Server Settings > Integrations` or just delete the webhook under `Channel Settings > Integrations > Webhooks`

## List Subreddits

To list all subreddit subscriptions run

```bash
/subreddit list
```
