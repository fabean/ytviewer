package ui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/fabean/ytviewer/internal/youtube"
)

// AppModel is the parent model that manages switching between views
type AppModel struct {
	youtubeClient *youtube.Client
	currentView   string
	videoModel    Model
	subModel      SubscriptionModel
}

// NewAppModel creates a new app model
func NewAppModel(client *youtube.Client) AppModel {
	return AppModel{
		youtubeClient: client,
		currentView:   "videos", // Start with video list
		videoModel:    NewModel(client),
		subModel:      NewSubscriptionModel(client),
	}
}

// Init initializes the app model
func (m AppModel) Init() tea.Cmd {
	return m.videoModel.Init()
}

// Update handles app model updates
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.currentView == "videos" && msg.String() == "s" {
			// Switch to subscription view
			m.currentView = "subscriptions"
			return m, m.subModel.Init()
		} else if m.currentView == "subscriptions" && msg.String() == "b" {
			// Switch back to video view
			m.currentView = "videos"
			return m, m.videoModel.Init()
		}
	}

	// Update the current view
	if m.currentView == "videos" {
		var cmd tea.Cmd
		videoModel, cmd := m.videoModel.Update(msg)
		if vm, ok := videoModel.(Model); ok {
			m.videoModel = vm
			cmds = append(cmds, cmd)
		}
	} else {
		var cmd tea.Cmd
		subModel, cmd := m.subModel.Update(msg)
		if sm, ok := subModel.(SubscriptionModel); ok {
			m.subModel = sm
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the current view
func (m AppModel) View() string {
	if m.currentView == "videos" {
		return m.videoModel.View()
	}
	return m.subModel.View()
} 