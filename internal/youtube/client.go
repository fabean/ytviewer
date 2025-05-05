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
	"github.com/google/uuid"
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
	channelCache        map[string]string // Map of channel ID to channel name
	videoCache          map[string][]Video // Map of channel ID to videos
	lastFetchTime       time.Time // When we last fetched videos
	cacheDuration       time.Duration // How long to cache videos for
	apiKey              string // Add this field to store the API key
}

// NewClient creates a new YouTube client
func NewClient(apiKey string, subscribedChannels []string, maxVideos int64, mpvOptions interface{}, cacheDuration int) (*Client, error) {
	ctx := context.Background()
	service, err := youtube.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("error creating YouTube client: %w", err)
	}

	client := &Client{
		service:            service,
		subscribedChannels: subscribedChannels,
		maxVideosPerChannel: maxVideos,
		channelCache:        make(map[string]string),
		videoCache:          make(map[string][]Video),
		lastFetchTime:       time.Time{}, // Zero time
		cacheDuration:       time.Duration(cacheDuration) * time.Minute,
		apiKey:              apiKey, // Store the API key
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
	// Check if cache is still valid
	if !c.lastFetchTime.IsZero() && time.Since(c.lastFetchTime) < c.cacheDuration {
		// Combine all videos from cache
		var allVideos []Video
		for _, videos := range c.videoCache {
			allVideos = append(allVideos, videos...)
		}
		
		// Sort by publish date (newest first)
		sort.Slice(allVideos, func(i, j int) bool {
			return allVideos[i].PublishedAt.After(allVideos[j].PublishedAt)
		})
		
		return allVideos, nil
	}
	
	// Cache expired or not initialized, fetch new videos
	allVideos := make([]Video, 0)
	
	// Process channels in batches to reduce API calls
	for i := 0; i < len(c.subscribedChannels); i += 50 {
		end := i + 50
		if end > len(c.subscribedChannels) {
			end = len(c.subscribedChannels)
		}
		
		batch := c.subscribedChannels[i:end]
		batchVideos, err := c.fetchVideosForChannels(batch)
		if err != nil {
			return nil, err
		}
		
		allVideos = append(allVideos, batchVideos...)
	}
	
	// Sort by publish date (newest first)
	sort.Slice(allVideos, func(i, j int) bool {
		return allVideos[i].PublishedAt.After(allVideos[j].PublishedAt)
	})
	
	// Update cache timestamp
	c.lastFetchTime = time.Now()
	
	return allVideos, nil
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

// GetChannelName fetches the name of a channel
func (c *Client) GetChannelName(channelID string) (string, error) {
	// Check if the channel name is in the cache
	if name, ok := c.channelCache[channelID]; ok {
		return name, nil
	}
	
	// If not in cache, fetch from API
	service, err := youtube.NewService(context.Background(), option.WithAPIKey(c.apiKey))
	if err != nil {
		return "", fmt.Errorf("error creating YouTube service: %w", err)
	}
	
	call := service.Channels.List([]string{"snippet"}).Id(channelID)
	response, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("error fetching channel: %w", err)
	}
	
	if len(response.Items) == 0 {
		return "", fmt.Errorf("channel not found")
	}
	
	// Store in cache and return
	channelName := response.Items[0].Snippet.Title
	c.channelCache[channelID] = channelName
	return channelName, nil
}

// GetSubscribedChannelNames fetches all subscribed channel names
func (c *Client) GetSubscribedChannelNames() (map[string]string, error) {
	result := make(map[string]string)
	var missingChannels []string
	
	// Check which channels we need to fetch
	for _, channelID := range c.subscribedChannels {
		if name, ok := c.channelCache[channelID]; ok {
			result[channelID] = name
		} else {
			missingChannels = append(missingChannels, channelID)
		}
	}
	
	// If all channels are cached, return immediately
	if len(missingChannels) == 0 {
		return result, nil
	}
	
	// Fetch missing channels in batches to reduce API calls
	service, err := youtube.NewService(context.Background(), option.WithAPIKey(c.apiKey))
	if err != nil {
		return result, fmt.Errorf("error creating YouTube service: %w", err)
	}
	
	// Process in batches of 50 (YouTube API limit)
	for i := 0; i < len(missingChannels); i += 50 {
		end := i + 50
		if end > len(missingChannels) {
			end = len(missingChannels)
		}
		
		batch := missingChannels[i:end]
		call := service.Channels.List([]string{"snippet"}).Id(strings.Join(batch, ","))
		response, err := call.Do()
		if err != nil {
			return result, fmt.Errorf("error fetching channels: %w", err)
		}
		
		// Add to cache and result
		for _, item := range response.Items {
			c.channelCache[item.Id] = item.Snippet.Title
			result[item.Id] = item.Snippet.Title
		}
	}
	
	return result, nil
}

// Add a new method to fetch videos for multiple channels at once
func (c *Client) fetchVideosForChannels(channelIDs []string) ([]Video, error) {
	var allVideos []Video
	
	// First, get all channel uploads playlist IDs in one API call
	service, err := youtube.NewService(context.Background(), option.WithAPIKey(c.apiKey))
	if err != nil {
		return nil, fmt.Errorf("error creating YouTube service: %w", err)
	}
	
	// Get channel details (including uploads playlist ID) in one API call
	channelsCall := service.Channels.List([]string{"contentDetails"}).Id(strings.Join(channelIDs, ","))
	channelsResponse, err := channelsCall.Do()
	if err != nil {
		return nil, fmt.Errorf("error fetching channels: %w", err)
	}
	
	// Process each channel's uploads playlist
	for _, channel := range channelsResponse.Items {
		channelID := channel.Id
		uploadsPlaylistID := channel.ContentDetails.RelatedPlaylists.Uploads
		
		// Fetch videos from uploads playlist
		playlistCall := service.PlaylistItems.List([]string{"snippet"}).
			PlaylistId(uploadsPlaylistID).
			MaxResults(c.maxVideosPerChannel)
		
		playlistResponse, err := playlistCall.Do()
		if err != nil {
			// Log error but continue with other channels
			fmt.Printf("Error fetching videos for channel %s: %v\n", channelID, err)
			continue
		}
		
		// Process videos
		channelVideos := make([]Video, 0, len(playlistResponse.Items))
		for _, item := range playlistResponse.Items {
			// Get channel name from cache if available
			channelName, ok := c.channelCache[channelID]
			if !ok {
				// If not in cache, use channel ID temporarily
				// We'll populate it later with the batch channel name fetch
				channelName = channelID
			}
			
			// Parse the published time
			publishedAt, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
			if err != nil {
				// Use current time as fallback
				publishedAt = time.Now()
			}
			
			video := Video{
				ID:          item.Snippet.ResourceId.VideoId,
				Title:       item.Snippet.Title,
				ChannelName: channelName,
				PublishedAt: publishedAt,
				Thumbnail:   item.Snippet.Thumbnails.Medium.Url,
			}
			
			channelVideos = append(channelVideos, video)
		}
		
		// Update video cache for this channel
		c.videoCache[channelID] = channelVideos
		allVideos = append(allVideos, channelVideos...)
	}
	
	// Now fetch any missing channel names in a single batch request
	var missingChannelIDs []string
	channelIDToVideos := make(map[string][]int) // Map channel ID to indices in allVideos
	
	for i, video := range allVideos {
		if video.ChannelName == video.ChannelName { // This is always true, but we need to check if it's a channel ID
			// Check if the channel name is actually a channel ID
			if _, err := uuid.Parse(video.ChannelName); err == nil || strings.HasPrefix(video.ChannelName, "UC") {
				missingChannelIDs = append(missingChannelIDs, video.ChannelName)
				indices := channelIDToVideos[video.ChannelName]
				channelIDToVideos[video.ChannelName] = append(indices, i)
			}
		}
	}
	
	// Deduplicate missing channel IDs
	missingChannelIDsMap := make(map[string]bool)
	for _, id := range missingChannelIDs {
		missingChannelIDsMap[id] = true
	}
	
	missingChannelIDs = make([]string, 0, len(missingChannelIDsMap))
	for id := range missingChannelIDsMap {
		missingChannelIDs = append(missingChannelIDs, id)
	}
	
	// Fetch missing channel names if needed
	if len(missingChannelIDs) > 0 {
		channelNames, err := c.GetChannelNamesForIDs(missingChannelIDs)
		if err != nil {
			// Log error but continue with channel IDs as names
			fmt.Printf("Error fetching channel names: %v\n", err)
		} else {
			// Update videos with channel names
			for channelID, indices := range channelIDToVideos {
				if name, ok := channelNames[channelID]; ok {
					// Update all videos for this channel
					for _, idx := range indices {
						allVideos[idx].ChannelName = name
					}
					
					// Also update in video cache
					if videos, ok := c.videoCache[channelID]; ok {
						for i := range videos {
							c.videoCache[channelID][i].ChannelName = name
						}
					}
				}
			}
		}
	}
	
	return allVideos, nil
}

// Add a method to get multiple channel names at once
func (c *Client) GetChannelNamesForIDs(channelIDs []string) (map[string]string, error) {
	result := make(map[string]string)
	var missingChannels []string
	
	// Check which channels we need to fetch
	for _, channelID := range channelIDs {
		if name, ok := c.channelCache[channelID]; ok {
			result[channelID] = name
		} else {
			missingChannels = append(missingChannels, channelID)
		}
	}
	
	// If all channels are cached, return immediately
	if len(missingChannels) == 0 {
		return result, nil
	}
	
	// Fetch missing channels in batches to reduce API calls
	service, err := youtube.NewService(context.Background(), option.WithAPIKey(c.apiKey))
	if err != nil {
		return result, fmt.Errorf("error creating YouTube service: %w", err)
	}
	
	// Process in batches of 50 (YouTube API limit)
	for i := 0; i < len(missingChannels); i += 50 {
		end := i + 50
		if end > len(missingChannels) {
			end = len(missingChannels)
		}
		
		batch := missingChannels[i:end]
		call := service.Channels.List([]string{"snippet"}).Id(strings.Join(batch, ","))
		response, err := call.Do()
		if err != nil {
			return result, fmt.Errorf("error fetching channels: %w", err)
		}
		
		// Add to cache and result
		for _, item := range response.Items {
			c.channelCache[item.Id] = item.Snippet.Title
			result[item.Id] = item.Snippet.Title
		}
	}
	
	return result, nil
}

// ClearVideoCache clears the video cache to force a fresh fetch
func (c *Client) ClearVideoCache() {
	c.videoCache = make(map[string][]Video)
	c.lastFetchTime = time.Time{} // Zero time
} 