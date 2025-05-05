package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

// Subscription represents a YouTube channel subscription
type Subscription struct {
	ID              string
	Title           string
	Description     string
	SubscriberCount uint64
	VideoCount      uint64
	Thumbnail       string
}

// Client handles YouTube API interactions
type Client struct {
	service            *youtube.Service
	subscribedChannels []string
	maxVideosPerChannel int64
	mpvOptions         struct {
		MaxResolution  string
		HardwareAccel  bool
		CacheSize      string
		MarkAsWatched  bool
	}
	cachedSubscriptions []Subscription // Add this field for caching
}

// NewClient creates a new YouTube client
func NewClient(apiKey string, subscribedChannels []string, maxVideos int64, mpvOptions interface{}) (*Client, error) {
	ctx := context.Background()
	service, err := youtube.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("error creating YouTube client: %w", err)
	}

	client := &Client{
		service:            service,
		subscribedChannels: subscribedChannels,
		maxVideosPerChannel: maxVideos,
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
func (c *Client) GetLatestVideos() ([]Video, error) {
	var videos []Video

	for _, channelID := range c.subscribedChannels {
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
			MaxResults(c.maxVideosPerChannel).
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

// GetSubscriptionInfo fetches detailed information about subscribed channels
func (c *Client) GetSubscriptionInfo() ([]Subscription, error) {
	// Check if we have cached subscription info
	if len(c.cachedSubscriptions) > 0 {
		return c.cachedSubscriptions, nil
	}

	// Check if there are any subscriptions
	if len(c.subscribedChannels) == 0 {
		return nil, fmt.Errorf("no subscriptions found")
	}

	var subscriptions []Subscription

	for _, channelID := range c.subscribedChannels {
		// Create a context with timeout for each request
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		
		// Get channel info
		channelResponse, err := c.service.Channels.List([]string{"snippet", "statistics"}).
			Id(channelID).
			Context(ctx).
			Do()
			
		// Cancel the context after the request is done
		cancel()
			
		if err != nil {
			return nil, fmt.Errorf("error fetching channel info: %w", err)
		}

		if len(channelResponse.Items) == 0 {
			continue
		}

		channel := channelResponse.Items[0]
		
		thumbnail := ""
		if thumbnails := channel.Snippet.Thumbnails; thumbnails != nil {
			if thumbnails.Medium != nil {
				thumbnail = thumbnails.Medium.Url
			} else if thumbnails.Default != nil {
				thumbnail = thumbnails.Default.Url
			}
		}

		subscriptions = append(subscriptions, Subscription{
			ID:              channelID,
			Title:           channel.Snippet.Title,
			Description:     channel.Snippet.Description,
			SubscriberCount: uint64(channel.Statistics.SubscriberCount),
			VideoCount:      uint64(channel.Statistics.VideoCount),
			Thumbnail:       thumbnail,
		})
	}

	// Sort subscriptions alphabetically by title
	sort.Slice(subscriptions, func(i, j int) bool {
		return strings.ToLower(subscriptions[i].Title) < strings.ToLower(subscriptions[j].Title)
	})

	// Cache the subscription info
	c.cachedSubscriptions = subscriptions
	
	return subscriptions, nil
}

// RemoveSubscription removes a channel from subscriptions
func (c *Client) RemoveSubscription(channelID string) error {
	// Find the index of the channel to remove
	index := -1
	for i, id := range c.subscribedChannels {
		if id == channelID {
			index = i
			break
		}
	}

	if index == -1 {
		return fmt.Errorf("channel not found in subscriptions")
	}

	// Remove the channel from the list
	c.subscribedChannels = append(c.subscribedChannels[:index], c.subscribedChannels[index+1:]...)
	
	// Clear the cache
	c.cachedSubscriptions = nil
	
	// Update the config file
	return c.saveSubscriptions()
}

// saveSubscriptions saves the updated subscription list to the config file
func (c *Client) saveSubscriptions() error {
	// Get config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error getting home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "ytviewer")
	configPath := filepath.Join(configDir, "config.json")
	
	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("error parsing config file: %w", err)
	}
	
	// Update subscriptions
	config["subscriptions"] = c.subscribedChannels
	
	// Write updated config
	updatedData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("error creating updated config: %w", err)
	}

	if err := os.WriteFile(configPath, updatedData, 0644); err != nil {
		return fmt.Errorf("error writing updated config: %w", err)
	}
	
	return nil
}

// AddSubscription adds a new channel to the subscriptions
func (c *Client) AddSubscription(channelID string) error {
	// Validate the channel ID
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Check if the channel exists
	channelResponse, err := c.service.Channels.List([]string{"snippet"}).
		Id(channelID).
		Context(ctx).
		Do()
		
	if err != nil {
		return fmt.Errorf("error checking channel: %w", err)
	}
	
	if len(channelResponse.Items) == 0 {
		return fmt.Errorf("channel not found")
	}
	
	// Check if already subscribed
	for _, subID := range c.subscribedChannels {
		if subID == channelID {
			return fmt.Errorf("already subscribed to this channel")
		}
	}
	
	// Add to subscriptions
	c.subscribedChannels = append(c.subscribedChannels, channelID)
	
	// Clear the cache so it will be refreshed
	c.cachedSubscriptions = nil
	
	// Save to config file
	err = c.saveSubscriptions()
	if err != nil {
		return fmt.Errorf("error saving config: %w", err)
	}
	
	return nil
} 