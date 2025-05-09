package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fabean/ytviewer/internal/config"
	"github.com/fabean/ytviewer/internal/ui"
	"github.com/fabean/ytviewer/internal/youtube"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check if API key is set
	if cfg.APIKey == "YOUR_YOUTUBE_API_KEY" {
		fmt.Println("Please set your YouTube API key in ~/.config/ytviewer/config.json")
		os.Exit(1)
	}

	// Create YouTube client with settings from config
	client, err := youtube.NewClient(
		cfg.APIKey, 
		cfg.Subscriptions, 
		cfg.MaxVideos, 
		cfg.MPVOptions,
		cfg.CacheDuration,
	)
	if err != nil {
		fmt.Printf("Error creating YouTube client: %v\n", err)
		os.Exit(1)
	}

	// Create and start the UI with the AppModel
	model := ui.NewAppModel(client)
	p := tea.NewProgram(model, tea.WithAltScreen())
	
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
} 