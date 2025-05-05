package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fabean/ytviewer/internal/youtube"
)

// Replace with your actual module path
// import "github.com/fabean/ytviewer/internal/youtube"

// Model represents the UI state
type Model struct {
	list         list.Model
	youtubeClient *youtube.Client
	videos       []youtube.Video
	loading      bool
	spinner      spinner.Model
	err          error
	width        int
	height       int
}

// Item represents a video in the list
type Item struct {
	video youtube.Video
}

// FilterValue returns the value to filter on
func (i Item) FilterValue() string {
	// Combine title and channel name for filtering with channel name repeated
	// to give it more weight in the search
	return i.video.Title + " " + i.video.ChannelName + " " + i.video.ChannelName
}

// Title returns the item title
func (i Item) Title() string {
	return i.video.Title
}

// Description returns the item description
func (i Item) Description() string {
	timeAgo := formatTimeAgo(i.video.PublishedAt)
	return fmt.Sprintf("%s • %s", 
		channelStyle.Render(i.video.ChannelName),
		dateStyle.Render(timeAgo))
}

// formatTimeAgo formats the time difference in a human-readable way
func formatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("%d minute%s ago", mins, pluralize(mins))
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("%d hour%s ago", hours, pluralize(hours))
	case diff < 30*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d day%s ago", days, pluralize(days))
	default:
		return t.Format("Jan 2, 2006")
	}
}

// pluralize returns "s" if n != 1
func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// NewModel creates a new UI model
func NewModel(client *youtube.Client) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Create a default delegate without trying to override the Render function
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedItemStyle
	delegate.Styles.SelectedDesc = selectedItemStyle

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "YouTube Subscriptions"
	l.Styles.Title = titleStyle
	
	// Use the same style for both pagination and help
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#25A065"))
	
	l.Styles.PaginationStyle = statusStyle
	l.Styles.HelpStyle = statusStyle
	
	// Make sure the pagination dots use the same style
	l.Styles.ActivePaginationDot = statusStyle.Copy()
	l.Styles.InactivePaginationDot = statusStyle.Copy()

	// Add custom keybindings for subscription management and video playback
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("s"),
				key.WithHelp("s", "manage subscriptions"),
			),
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "play video"),
			),
			key.NewBinding(
				key.WithKeys("r"),
				key.WithHelp("r", "reload videos"),
			),
			key.NewBinding(
				key.WithKeys("f"),
				key.WithHelp("f", "force reload (clear cache)"),
			),
		}
	}

	return Model{
		list:         l,
		youtubeClient: client,
		loading:      true,
		spinner:      s,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchVideos(),
	)
}

// fetchVideos fetches videos from YouTube
func (m Model) fetchVideos() tea.Cmd {
	return func() tea.Msg {
		// Get videos from the YouTube client
		videos, err := m.youtubeClient.GetLatestVideos()
		if err != nil {
			return errMsg{err}
		}
		
		return videosMsg{videos}
	}
}

// Update handles UI updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
			// Switch to subscription manager
			subModel := NewSubscriptionModel(m.youtubeClient)
			return subModel, subModel.Init()

		case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
			// Regular reload (uses cache if valid)
			m.loading = true
			return m, tea.Batch(
				m.spinner.Tick,
				m.fetchVideos(),
			)
			
		case key.Matches(msg, key.NewBinding(key.WithKeys("f"))):
			// Force reload (clear cache)
			m.youtubeClient.ClearVideoCache()
			m.loading = true
			return m, tea.Batch(
				m.spinner.Tick,
				m.fetchVideos(),
			)

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if m.list.SelectedItem() != nil {
				selectedItem := m.list.SelectedItem().(Item)
				return m, func() tea.Msg {
					err := m.youtubeClient.PlayVideo(selectedItem.video.ID)
					if err != nil {
						return errMsg{err}
					}
					return nil
				}
			}
		}

	case videosMsg:
		m.videos = msg.videos
		m.loading = false
		
		// Convert videos to list items
		items := make([]list.Item, len(m.videos))
		for i, video := range m.videos {
			items[i] = Item{video: video}
		}
		
		m.list.SetItems(items)

	case errMsg:
		m.err = msg.err
		m.loading = false

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case returnToMainMsg:
		// Reset the model to loading state
		m.loading = true
		return m, tea.Batch(
			m.spinner.Tick,
			m.fetchVideos(),
		)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress q to quit.", m.err)
	}

	if m.loading {
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			lipgloss.JoinVertical(
				lipgloss.Center,
				m.spinner.View()+" Loading videos...",
				"",
				"Press q to quit",
			),
		)
	}

	return listStyle.Render(m.list.View())
}

// Message types
type videosMsg struct {
	videos []youtube.Video
}

type errMsg struct {
	err error
}

func (e errMsg) Error() string {
	return e.err.Error()
}

// Add a new method to handle returning from subscription manager
func (m Model) ReturnFromSubscriptions() tea.Cmd {
	// Reset the model to loading state
	m.loading = true
	
	// Return commands to start the spinner and fetch videos
	return tea.Batch(
		m.spinner.Tick,
		m.fetchVideos(),
	)
} 