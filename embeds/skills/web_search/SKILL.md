---
name: web_search
description: Search the web for how-tos, articles, news, and up-to-date information
---

# Web Search

Use the Brave Search API via the `web` tool when the user asks for how-tos, articles, news, or current information.

## Setup

Requires a Brave Search API key. The user must store it in `.brave_api_key` in this skill's directory (e.g. `~/.picobot/workspace/skills/web_search/.brave_api_key`). Sign up at [api.search.brave.com](https://api.search.brave.com) (free tier: 2,000 requests/month).

## How to Search

1. Read the API key: `filesystem(action="read", path="skills/web_search/.brave_api_key")`
2. Call the web tool with the search URL and auth header. URL-encode the query (spaces as `+` or `%20`):

```
web(url="https://api.search.brave.com/res/v1/web/search?q=SEARCH_QUERY", headers={"X-Subscription-Token": "<key from step 1>"})
```

Example for "python async tutorial":

```
web(url="https://api.search.brave.com/res/v1/web/search?q=python+async+tutorial", headers={"X-Subscription-Token": "<key>"})
```

## Query Parameters

Append to the URL for more control:

| Param | Value | Description |
|-------|-------|-------------|
| `freshness` | `pd` | Past 24 hours |
| `freshness` | `pw` | Past week |
| `freshness` | `pm` | Past month |
| `freshness` | `py` | Past year |
| `count` | `10` | Number of results (default 20) |

Example for recent news (past week):

```
web(url="https://api.search.brave.com/res/v1/web/search?q=SEARCH_QUERY&freshness=pw", headers={"X-Subscription-Token": "<key>"})
```

## Response Format

JSON response under `web.results[]`. Each result has:

- `title` — page title
- `url` — page URL
- `description` — snippet/excerpt

Use the `web` tool to fetch full content from result URLs if the user needs more detail.

## When to Use

- User asks how to do something (tutorials, guides)
- User wants articles, news, or recent information
- User needs up-to-date facts not in your training data
