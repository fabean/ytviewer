# ytviewer
YouTube TUI that pulls subscriptions from YouTube API and plays videos in MPV

![ytviewer demo](https://github.com/fabean/ytviewer/raw/main/ytviewer.gif)

## Overview

ytviewer is a terminal-based YouTube subscription viewer built in Go using the Bubble Tea framework. It allows you to:

- View the latest videos from your subscribed channels
- Play videos directly in MPV with optimized settings
- Navigate your subscriptions with a simple keyboard interface

## Installation

### Prerequisites

- Go 1.16 or higher
- MPV media player
- YouTube API key

### Building from source

```bash
# Clone the repository
git clone https://github.com/fabean/ytviewer.git
cd ytviewer

# Build the application
go build -o ytviewer cmd/ytviewer/main.go

# Optional: Install system-wide
go install ./cmd/ytviewer
```
## Configuration

ytviewer requires a configuration file at `~/.config/ytviewer/config.json` with the following structure:

```json
{
  "api_key": "YOUR_YOUTUBE_API_KEY",
  "subscriptions": [
    "CHANNEL_ID_1",
    "CHANNEL_ID_2"
  ]
}
```

### Getting a YouTube API Key

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Navigate to "APIs & Services" > "Library"
4. Search for and enable "YouTube Data API v3"
5. Go to "APIs & Services" > "Credentials"
6. Click "Create Credentials" > "API Key"
7. Copy the generated API key to your config file

### Finding YouTube Channel IDs

To find a channel ID:

1. Go to the YouTube channel page
2. View the page source (right-click > "View Page Source")
3. Search for "channelId"
4. The ID will look like "UC0intLFzLaudFG-xAvUEO-A"

Alternatively, use a service like [Comment Picker](https://commentpicker.com/youtube-channel-id.php) to find channel IDs.

## Usage

```bash
# Run the application
ytviewer
```

### Keyboard Controls

- `↑`/`↓`: Navigate through videos
- `Enter`: Play selected video in MPV
- `q`: Quit the application

## Features

- Fetches latest videos from your subscribed channels
- Displays video titles, channel names, and publish dates
- Plays videos in MPV with optimized settings (1080p max resolution)
- Simple, keyboard-driven interface

## Notes

- The YouTube Data API has quotas (10,000 units per day for free tier)
- Different API operations consume different amounts of quota
- The application requires a valid YouTube API key and at least one channel ID in the config file to work


