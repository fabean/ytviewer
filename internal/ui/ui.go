package ui

import (
	"fmt"
	"io"
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
	watched bool
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

// CustomDelegate extends the default delegate with custom rendering
type CustomDelegate struct {
	list.DefaultDelegate
	bulletStyle lipgloss.Style
}

// Render overrides the default render method to add a bullet for selected items
func (d CustomDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	// Add bullet or space at the beginning with reduced spacing
	if index == m.Index() {
		fmt.Fprint(w, d.bulletStyle.Render("●"))
	} else {
		fmt.Fprint(w, " ")
	}
	
	// Get the item
	item, ok := listItem.(Item)
	if !ok {
		return
	}
	
	// Render title with proper styling
	title := item.Title()
	if index == m.Index() {
		title = d.Styles.SelectedTitle.Render(title)
	} else {
		title = d.Styles.NormalTitle.Render(title)
	}
	
	// Add watched indicator if the video has been watched
	if item.watched {
		watchedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
		title = title + " " + watchedStyle.Render("✓")
	}
	
	fmt.Fprintln(w, title)
	
	// Render description with proper indentation and styling
	desc := item.Description()
	if desc != "" {
		if index == m.Index() {
			desc = d.Styles.SelectedDesc.Render(desc)
		} else {
			desc = d.Styles.NormalDesc.Render(desc)
		}
		// Add the same indentation for the description line
		fmt.Fprintf(w, " %s", desc)
	}
	// No extra newline at the end - let the list handle spacing
}

// NewModel creates a new UI model
func NewModel(client *youtube.Client) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Create a default delegate
	defaultDelegate := list.NewDefaultDelegate()
	
	// Remove highlighting from selected items
	defaultDelegate.Styles.SelectedTitle = defaultDelegate.Styles.NormalTitle.Copy()
	defaultDelegate.Styles.SelectedDesc = defaultDelegate.Styles.NormalDesc.Copy()
	
	// Create our custom delegate with bullet styling
	bulletStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#25A065"))
	delegate := CustomDelegate{
		DefaultDelegate: defaultDelegate,
		bulletStyle:     bulletStyle,
	}
	// Set spacing to 1 to add space between items
	delegate.SetSpacing(1)

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
					// Mark the video as watched before playing it
					err := m.youtubeClient.MarkVideoAsWatched(selectedItem.video.ID)
					if err != nil {
						return errMsg{err}
					}
					
					err = m.youtubeClient.PlayVideo(selectedItem.video.ID)
					if err != nil {
						return errMsg{err}
					}
					return videoWatchedMsg{videoID: selectedItem.video.ID}
				}
			}
		}

	case videosMsg:
		m.videos = msg.videos
		m.loading = false
		
		// Get watched videos
		watchedVideos, err := m.youtubeClient.GetWatchedVideos()
		if err != nil {
			m.err = err
			m.loading = false
			break
		}
		
		// Convert videos to list items
		items := make([]list.Item, len(m.videos))
		for i, video := range m.videos {
			// Check if this video is in the watched list
			watched := watchedVideos[video.ID]
			items[i] = Item{video: video, watched: watched}
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

	case videoWatchedMsg:
		// Update the watched status in the list
		for i, item := range m.list.Items() {
			videoItem, ok := item.(Item)
			if ok && videoItem.video.ID == msg.videoID {
				videoItem.watched = true
				m.list.SetItem(i, videoItem)
				break
			}
		}
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

// Add a new message type for when a video is watched
type videoWatchedMsg struct {
	videoID string
}
