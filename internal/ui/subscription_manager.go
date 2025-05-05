package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fabean/ytviewer/internal/youtube"
)

// SubscriptionModel represents the subscription manager UI state
type SubscriptionModel struct {
	youtubeClient *youtube.Client
	subscriptions []youtube.Subscription
	loading       bool
	spinner       spinner.Model
	err           error
	width         int
	height        int
	cursor        int
	offset        int
	
	// Add mode state
	addMode     bool
	channelInput textinput.Model
	addError    string
}

// formatNumber formats a number with commas (e.g., 1,234,567)
func formatNumber(n uint64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	} else if n < 1000000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d,%03d,%03d", n/1000000, (n/1000)%1000, n%1000)
}

// NewSubscriptionModel creates a new subscription manager model
func NewSubscriptionModel(client *youtube.Client) SubscriptionModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	
	// Initialize text input for channel ID
	ti := textinput.New()
	ti.Placeholder = "Enter YouTube Channel ID"
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 30

	return SubscriptionModel{
		youtubeClient: client,
		loading:       true,
		spinner:       s,
		cursor:        0,
		offset:        0,
		channelInput:  ti,
		addMode:       false,
	}
}

// Init initializes the subscription manager model
func (m SubscriptionModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchSubscriptions(),
	)
}

// fetchSubscriptions fetches channel information for subscriptions
func (m SubscriptionModel) fetchSubscriptions() tea.Cmd {
	return func() tea.Msg {
		subscriptions, err := m.youtubeClient.GetSubscriptionInfo()
		if err != nil {
			return errMsg{err}
		}
		
		return subscriptionsMsg{subscriptions: subscriptions}
	}
}

// Update handles UI updates for the subscription manager
func (m SubscriptionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// If in add mode, handle input differently
		if m.addMode {
			switch msg.String() {
			case "esc":
				// Cancel add mode
				m.addMode = false
				m.addError = ""
				return m, nil
				
			case "enter":
				// Try to add the channel
				channelID := strings.TrimSpace(m.channelInput.Value())
				if channelID == "" {
					m.addError = "Channel ID cannot be empty"
					return m, nil
				}
				
				// Exit add mode and start adding the channel
				m.addMode = false
				m.loading = true
				m.addError = ""
				
				return m, func() tea.Msg {
					err := m.youtubeClient.AddSubscription(channelID)
					if err != nil {
						return errMsg{err}
					}
					
					// Refresh subscriptions after adding
					subscriptions, err := m.youtubeClient.GetSubscriptionInfo()
					if err != nil {
						return errMsg{err}
					}
					
					return subscriptionsMsg{subscriptions: subscriptions}
				}
			}
			
			// Handle text input
			var cmd tea.Cmd
			m.channelInput, cmd = m.channelInput.Update(msg)
			return m, cmd
		}
		
		// Normal mode key handling
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
			
		case "b":
			// Return to main view
			return NewModel(m.youtubeClient), tea.Batch(
				func() tea.Msg { return loadMainViewMsg{} },
			)

		case "a":
			// Enter add mode
			m.addMode = true
			m.channelInput.Reset()
			m.channelInput.Focus()
			return m, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.subscriptions)-1 {
				m.cursor++
			}
			
		case "home":
			m.cursor = 0
			
		case "end":
			m.cursor = len(m.subscriptions) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
			
		case "pgup":
			// Move up by 10 items
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
			
		case "pgdown":
			// Move down by 10 items
			m.cursor += 10
			if m.cursor >= len(m.subscriptions) {
				m.cursor = len(m.subscriptions) - 1
				if m.cursor < 0 {
					m.cursor = 0
				}
			}

		case "d":
			// Unsubscribe from selected channel
			if len(m.subscriptions) > 0 && m.cursor < len(m.subscriptions) {
				selectedChannel := m.subscriptions[m.cursor]
				return m, func() tea.Msg {
					err := m.youtubeClient.RemoveSubscription(selectedChannel.ID)
					if err != nil {
						return errMsg{err}
					}
					return unsubscribedMsg{channelID: selectedChannel.ID}
				}
			}
		}

	case subscriptionsMsg:
		m.subscriptions = msg.subscriptions
		m.loading = false

	case unsubscribedMsg:
		// Remove the unsubscribed channel from the subscriptions
		var newSubscriptions []youtube.Subscription
		for _, sub := range m.subscriptions {
			if sub.ID != msg.channelID {
				newSubscriptions = append(newSubscriptions, sub)
			}
		}
		m.subscriptions = newSubscriptions
		
		// Adjust cursor if needed
		if m.cursor >= len(m.subscriptions) {
			m.cursor = len(m.subscriptions) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
		}

	case errMsg:
		m.err = msg.err
		m.loading = false

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the subscription manager UI
func (m SubscriptionModel) View() string {
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
				m.spinner.View()+" Loading subscriptions...",
				"",
				"Press q to quit",
			),
		)
	}
	
	// If in add mode, show the add channel form
	if m.addMode {
		var sb strings.Builder
		
		title := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Render("Add Channel by ID")
		
		sb.WriteString(title)
		sb.WriteString("\n\n")
		
		sb.WriteString("Enter YouTube Channel ID:\n")
		sb.WriteString(m.channelInput.View())
		sb.WriteString("\n\n")
		
		if m.addError != "" {
			errorText := lipgloss.NewStyle().
				Foreground(lipgloss.Color("9")).
				Render(m.addError)
			sb.WriteString(errorText)
			sb.WriteString("\n\n")
		}
		
		help := "Press Enter to add • Esc to cancel"
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(help))
		
		return lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1).
			Render(sb.String())
	}

	// Set a fixed number of visible items (25)
	maxVisible := 25
	
	// Calculate start and end indices for pagination
	startIdx := 0
	if len(m.subscriptions) > maxVisible {
		// Center the cursor in the visible window when possible
		halfVisible := maxVisible / 2
		startIdx = m.cursor - halfVisible
		
		// Adjust if we're near the beginning
		if startIdx < 0 {
			startIdx = 0
		}
		
		// Adjust if we're near the end
		if startIdx > len(m.subscriptions) - maxVisible {
			startIdx = len(m.subscriptions) - maxVisible
		}
	}
	
	endIdx := startIdx + maxVisible
	if endIdx > len(m.subscriptions) {
		endIdx = len(m.subscriptions)
	}
	
	visibleSubs := m.subscriptions[startIdx:endIdx]
	
	// Build the view
	var sb strings.Builder
	
	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("Channel Subscriptions")
	
	sb.WriteString(title)
	sb.WriteString("\n\n")
	
	// Subscriptions
	for i, sub := range visibleSubs {
		idx := i + startIdx
		
		// Format subscriber count
		subCount := "Unknown"
		if sub.SubscriberCount > 0 {
			subCount = formatNumber(sub.SubscriberCount) + " subscribers"
		}
		
		// Style based on selection
		var line string
		if idx == m.cursor {
			// Selected style
			channelName := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205")).
				Background(lipgloss.Color("57")).
				Render(sub.Title)
				
			subscriberInfo := lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Background(lipgloss.Color("57")).
				Render(subCount)
				
			line = fmt.Sprintf("> %s - %s", channelName, subscriberInfo)
		} else {
			// Normal style
			channelName := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205")).
				Render(sub.Title)
				
			subscriberInfo := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Render(subCount)
				
			line = fmt.Sprintf("  %s - %s", channelName, subscriberInfo)
		}
		
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	
	// Pagination info
	pagination := fmt.Sprintf("\n[%d/%d]", m.cursor+1, len(m.subscriptions))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(pagination))
	
	// Help text
	help := "\nup/down: navigate • a: add channel • d: unsubscribe • b: back • q: quit"
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(help))
	
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1).
		Render(sb.String())
}

// Message types
type subscriptionsMsg struct {
	subscriptions []youtube.Subscription
}

type unsubscribedMsg struct {
	channelID string
}

type loadMainViewMsg struct{} 