package youtube

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// Video represents a YouTube video
type Video struct {
	ID          string
	Title       string
	ChannelName string
	PublishedAt time.Time
	Thumbnail   string
}

// Client handles YouTube API interactions
type Client struct {
	service           *youtube.Service
	subscribedChannels []string
	mpvOptions        struct {
		MaxResolution  string
		HardwareAccel  bool
		CacheSize      string
		MarkAsWatched  bool
	}
}

// NewClient creates a new YouTube client
func NewClient(apiKey string, subscribedChannels []string, mpvOptions interface{}) (*Client, error) {
	ctx := context.Background()
	service, err := youtube.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("error creating YouTube client: %w", err)
	}

	client := &Client{
		service:           service,
		subscribedChannels: subscribedChannels,
	}
	
	// Set MPV options if provided
	if mpvOptions != nil {
		if opts, ok := mpvOptions.(struct {
			MaxResolution  string
			HardwareAccel  bool
			CacheSize      string
			MarkAsWatched  bool
		}); ok {
			client.mpvOptions = opts
		}
	}
	
	return client, nil
}

// GetSubscribedChannels returns the list of subscribed channel IDs
func (c *Client) GetSubscribedChannels() []string {
	return c.subscribedChannels
}

// GetLatestVideos fetches the latest videos from the subscribed channels
func (c *Client) GetLatestVideos(channelIDs []string, maxResults int64) ([]Video, error) {
	var videos []Video

	for _, channelID := range channelIDs {
		// Get channel info to get the uploads playlist ID
		channelResponse, err := c.service.Channels.List([]string{"contentDetails", "snippet"}).
			Id(channelID).
			MaxResults(1).
			Do()
		if err != nil {
			return nil, fmt.Errorf("error fetching channel info: %w", err)
		}

		if len(channelResponse.Items) == 0 {
			continue
		}

		channelName := channelResponse.Items[0].Snippet.Title
		uploadsPlaylistID := channelResponse.Items[0].ContentDetails.RelatedPlaylists.Uploads

		// Get videos from the uploads playlist
		playlistResponse, err := c.service.PlaylistItems.List([]string{"snippet", "contentDetails"}).
			PlaylistId(uploadsPlaylistID).
			MaxResults(maxResults).
			Do()
		if err != nil {
			return nil, fmt.Errorf("error fetching playlist items: %w", err)
		}

		for _, item := range playlistResponse.Items {
			publishedAt, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
			
			thumbnail := ""
			if thumbnails := item.Snippet.Thumbnails; thumbnails != nil {
				if thumbnails.Medium != nil {
					thumbnail = thumbnails.Medium.Url
				} else if thumbnails.Default != nil {
					thumbnail = thumbnails.Default.Url
				}
			}

			videos = append(videos, Video{
				ID:          item.Snippet.ResourceId.VideoId,
				Title:       item.Snippet.Title,
				ChannelName: channelName,
				PublishedAt: publishedAt,
				Thumbnail:   thumbnail,
			})
		}
	}

	// Sort videos by published date (newest first)
	// This is a simple implementation; you might want to improve it
	for i := 0; i < len(videos); i++ {
		for j := i + 1; j < len(videos); j++ {
			if videos[i].PublishedAt.Before(videos[j].PublishedAt) {
				videos[i], videos[j] = videos[j], videos[i]
			}
		}
	}

	return videos, nil
}

// PlayVideo opens the video in MPV with optimized settings
func (c *Client) PlayVideo(videoID string) error {
	url := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	
	// Basic MPV arguments that should work reliably
	args := []string{
		// Limit resolution to 1080p
		"--ytdl-format=bestvideo[height<=1080]+bestaudio/best[height<=1080]",
		
		// The video URL (must be the last argument)
		url,
	}
	
	// Create and start the MPV process
	cmd := exec.Command("mpv", args...)
	
	// For debugging: print the command being executed
	fmt.Printf("Executing: mpv %s\n", strings.Join(args, " "))
	
	// Start MPV
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error starting MPV: %v\n", err)
	}
	
	return err
} 