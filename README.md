# Railway Assistant

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/railway-assistant?referralCode=1uJ_oA)

A production-ready webhook service designed to integrate Railway with Telegram. It acts as a bridge, receiving webhook events from your Railway projects and forwarding them as formatted notifications to one or more Telegram chats, groups, or channels per Railway project.

## Features

- **Real-time Notifications**: Get instant alerts for deployments, build failures, and service crashes via Telegram and Slack.
- **Project Routing**: Route each Railway project to different Telegram destinations.
- **Bot-managed Settings**: Configure routes from Telegram using `/projects`, `/connect`, `/routes`, `/disconnect`, and `/test`.
- **Durable Config**: Store routing settings in a JSON config file on a Railway Volume, preserving settings across redeploys.
- **Smart Formatting**: Messages are cleanly formatted with Markdown, including status emojis and direct links to your Railway projects, services, and deployments.
- **Configurable Detail**: Control exactly what information is included in your notifications (Workspace, Branch, Commit, Author, etc.).
- **Zero Dependencies**: Built with Go standard library only. No external frameworks or bloat.
- **Ultra-Lightweight**: Designed for minimal compute resource usage and instant startup.

## Setup Guide

### 1. Create a Telegram Bot or Slack App

**Telegram:**

1.  Open Telegram and search for **@BotFather**.
2.  Send the command `/newbot`.
3.  Follow the prompts to name your bot.
4.  Copy the **HTTP API Token** provided (this is your `TELEGRAM_BOT_TOKEN`).
5.  Start a private chat with your new bot.
6.  Get your own Telegram user ID. This ID goes into `ADMIN_TELEGRAM_USER_IDS` so only you can manage routing.
7.  Add the bot to each group/channel that should receive notifications.

**Slack:**

1.  Create a Slack App at [api.slack.com/apps](https://api.slack.com/apps).
2.  Enable **Incoming Webhooks** for your app.
3.  Create a new Webhook URL for the channel you want to post to.
4.  Copy the **Webhook URL** (this is your `SLACK_WEBHOOK_URL`).

### 2. Attach Persistent Storage

For bot-managed routing, attach a Railway Volume to the service. Railway will provide `RAILWAY_VOLUME_MOUNT_PATH`, and the app will store config at:

```text
${RAILWAY_VOLUME_MOUNT_PATH}/railway-assistant/config.json
```

You can override this with `CONFIG_PATH`.

### 3. Configure Environment Variables

Runtime secrets and startup options are configured via environment variables. Project routing is configured through the Telegram bot and stored in the durable config file.

**Required:**

| Variable                  | Description                                      |
| :------------------------ | :----------------------------------------------- |
| `PORT`                    | The port to listen on (default: `8080`).         |
| `TELEGRAM_BOT_TOKEN`      | The API token you got from BotFather.            |
| `ADMIN_TELEGRAM_USER_IDS` | Comma-separated Telegram user IDs allowed to manage routes. |
| `TELEGRAM_WEBHOOK_SECRET` | Secret used to verify Telegram webhook delivery. |

**Recommended:**

| Variable                | Description |
| :---------------------- | :---------- |
| `PUBLIC_BASE_URL`       | Public service URL. If set, the app registers `/telegram/webhook` on startup. |
| `RAILWAY_WEBHOOK_TOKEN` | Optional shared secret for Railway webhooks. Send it as `X-Railway-Webhook-Token` or `?token=...`. |
| `CONFIG_PATH`           | Optional explicit config path. Defaults to the Railway Volume path above. |

**Legacy / Optional Provider Settings:**

| Variable             | Description |
| :------------------- | :---------- |
| `TELEGRAM_ENABLED`   | Legacy flag for single-chat Telegram notifications. |
| `TELEGRAM_CHAT_ID`   | Legacy fallback chat used when no project route matches. |
| `SLACK_ENABLED`      | Set to `true` to enable global Slack notifications. |
| `SLACK_WEBHOOK_URL`  | The Slack incoming webhook URL. |

**Optional (Message Customization):**

Control what information appears in your alerts by setting these to `true` or `false` (default is `true`).

| Variable            | Default | Description                                              |
| :------------------ | :------ | :------------------------------------------------------- |
| `INCLUDE_WORKSPACE` | `true`  | Show the Workspace name.                                 |
| `INCLUDE_STATUS`    | `true`  | Show the deployment status (e.g., INITIALIZING, FAILED). |
| `INCLUDE_BRANCH`    | `true`  | Show the git branch name.                                |
| `INCLUDE_COMMIT`    | `true`  | Show the commit message.                                 |
| `INCLUDE_AUTHOR`    | `true`  | Show the commit author.                                  |

### 4. Deploy & Connect

1.  **Deploy this template** to your own Railway project.
2.  Attach a Railway Volume if you are using bot-managed routing.
3.  Set the required Environment Variables in your new service.
4.  If `PUBLIC_BASE_URL` is not set, manually register the Telegram webhook at:
    - `https://<YOUR_SERVICE_URL>/telegram/webhook`
    - Include `secret_token=<TELEGRAM_WEBHOOK_SECRET>` in the Telegram `setWebhook` call.
5.  Send `/status` to the bot in private chat. It should show config and route counts.
6.  Add a Railway webhook for each project:
    - **Payload URL**: `https://<YOUR_SERVICE_URL>/railway/alerts`
    - If `RAILWAY_WEBHOOK_TOKEN` is set, use `https://<YOUR_SERVICE_URL>/railway/alerts?token=<TOKEN>` or configure the `X-Railway-Webhook-Token` header.
    - **Event Types**: Select the events you want to be notified about.
7.  Trigger Railway's test webhook, then send `/projects` to the bot.
8.  In the target Telegram group, run `/connect <project_id>`. For channels, run `/connect <project_id> <channel_chat_id>` in private chat after adding the bot as a channel admin.
9.  Run `/test <project_id>` to verify delivery.

Bot commands:

| Command | Description |
| :------ | :---------- |
| `/status` | Show config path and counts. |
| `/projects` | List Railway projects seen from webhooks. |
| `/routes` | List configured routes. |
| `/connect <project_id>` | Connect the current group or supergroup to a project. |
| `/connect <project_id> <chat_id>` | Connect a chat or channel from private chat. |
| `/disconnect <project_id>` | Disconnect the current group, or remove the whole project route in private chat. |
| `/test <project_id>` | Send a test notification through the route. |

## License

MIT

## Example Notification

Here is an example of what a notification looks like in Telegram:

🚄 _Railway Alert_

🔵 _DEPLOYED_

_Details_
📍 My Team Workspace / My Project / Frontend App
🌍 Production
🔄 Status: Success

_Git_
💬 _feat: add new dashboard components_
🌱 main • 👤 username
