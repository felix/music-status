# Slack music status

Sets your slack status from MPD.

## Slack authorization

- It requires a personal token ie. starting with `xoxp-`

## Usage

```
Usage of ./mpd-slack-status:
  -default-emoji string
        Default status emoji
  -default-text string
        Default status text
  -expire-status
        Set status expiry, approximately 30s
  -mpd-address string
        MPD address (default "127.0.0.1:6600")
  -mpd-password string
        MPD password
  -slack-token string
        Slack API token
  -slack-url string
        Base URL for your Slack team
```
