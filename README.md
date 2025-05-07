# ytviewer
YouTube TUI that pulls subscriptions from YouTube API and plays videos in MPV

![ytviewer demo](https://github.com/fabean/ytviewer/raw/main/ytviewer.gif)

## Overview

ytviewer is a terminal-based YouTube subscription viewer built in Go using the Bubble Tea framework. It allows you to:

- View the latest videos from your subscribed channels
- Play videos directly in MPV with optimized settings
- Navigate your subscriptions with a simple keyboard interface
- Manage your subscriptions directly through the TUI
- Filter videos by title or channel name
- Track watched videos with persistent history
- Copy video URLs to clipboard

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
  ],
  "max_videos": 10,
  "mpv_options": {
    "max_resolution": "1080",
    "hardware_accel": true,
    "cache_size": "150M",
    "mark_as_watched": true
  },
  "cache_duration": 30
}
```

- **api_key**: Your YouTube API key
- **subscriptions**: List of YouTube channel IDs
- **max_videos**: Maximum number of videos to fetch per channel
- **mpv_options**: Options for the MPV player
  - **max_resolution**: Maximum video resolution
  - **hardware_accel**: Enable hardware acceleration
  - **cache_size**: MPV cache size
  - **mark_as_watched**: Mark videos as watched after playing
- **cache_duration**: How long to cache videos (in minutes)

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
4. The ID will look like "UCuAXFkgsw1L7xaCfnd5JJOw"

Alternatively, use a service like [Comment Picker](https://commentpicker.com/youtube-channel-id.php) to find channel IDs.

## Usage

```bash
# Run the application
ytviewer
```

### Keyboard Controls

#### Main View
- `/`: Filter videos (by title or channel name)
- `↑`/`↓`: Navigate through videos
- `Enter`: Play selected video in MPV
- `c`: Copy current video URL to clipboard
- `s`: Open subscription management screen
- `r`: Reload videos (uses cache if valid)
- `f`: Force reload videos (clears cache)
- `q`: Quit the application

#### Subscription Management
- `↑`/`↓`: Navigate through subscriptions
- `a`: Add new subscription by entering a channel ID
- `d`: Remove selected subscription
- `b`: Return to main video list
- `q`: Quit the application

### Managing Subscriptions

Press `s` from the main screen to access the subscription management interface. From there, you can:

- View all your current subscriptions
- Add new subscriptions by entering a channel ID
- Remove existing subscriptions
- Return to the main video list

Changes to subscriptions are automatically saved to your config file.

### Video Reloading and Caching

To minimize API usage and improve performance, ytviewer implements caching:

- **r**: Reload videos from cache (if available and not expired)
- **f**: Force reload by clearing all caches and fetching fresh data from YouTube API

The cache duration is configurable in your config file using the `cache_duration` setting (in minutes). The default is 30 minutes.

## Features

- Fetches latest videos from your subscribed channels
- Displays video titles, channel names, and publish dates
- Plays videos in MPV with optimized settings
- Simple, keyboard-driven interface
- Manage subscriptions directly through the TUI
- Filter videos by title or channel name
- Caching to reduce API usage
- Persistent watch history tracking
- Copy video URLs to clipboard

## Notes

- The YouTube Data API has quotas (10,000 units per day for free tier)
- Different API operations consume different amounts of quota
- The application requires a valid YouTube API key and at least one channel ID in the config file to work
- Using the cache functionality can help stay within API limits
