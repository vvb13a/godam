package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vvb13a/godam/internal/domain"
	"github.com/vvb13a/godam/internal/service"
)

type focusPane int

const (
	focusSidebar focusPane = iota
	focusTable
)

type viewMode int

const (
	viewModeList viewMode = iota
	viewModeGrid
)

type modalType int

const (
	modalNone modalType = iota
	modalCreateCollection
	modalRenameCollection
	modalUpload
	modalAddTag
	modalAddToCollection
	modalConfirmDelete
	modalZoomPreview
)

type deleteTargetType int

const (
	deleteAsset deleteTargetType = iota
	deleteCollection
)

// Messages
type (
	collectionsLoadedMsg []domain.Collection
	assetsLoadedMsg      []domain.Asset
	uploadProgressMsg    float64
	uploadCompleteMsg    struct {
		asset *domain.Asset
		err   error
	}
	previewLoadedMsg struct {
		assetID string
		preview string
	}
	cardPreviewLoadedMsg struct {
		assetID string
		preview string
	}
	zoomPreviewLoadedMsg string
	errMsg               error
)

type Model struct {
	assetService      *service.AssetService
	collectionService *service.CollectionService

	focusedPane           focusPane
	activeViewMode        viewMode
	activeModal           modalType
	collections           []domain.Collection
	selectedCollectionIdx int
	assets                []domain.Asset

	// Grid state
	gridCursor int
	gridCols   int

	// History Pane Toggle
	showHistory bool

	// Inputs & Modals
	textInput        textinput.Model
	editingColID     string
	pickerSelected   int
	deleteTarget     deleteTargetType
	deleteTargetID   string
	deleteTargetName string

	// Previews
	previewCache     map[string]string
	cardPreviewCache map[string]string
	currentPreview   string
	zoomPreview      string

	// History
	history []string

	// Upload Streaming
	progressChan chan float64
	doneChan     chan uploadCompleteMsg
	progress     progress.Model
	isUploading  bool

	table  table.Model
	width  int
	height int
	err    error
}

func NewModel(assetSvc *service.AssetService, colSvc *service.CollectionService) Model {
	ti := textinput.New()
	ti.CharLimit = 150

	pg := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(30),
		progress.WithoutPercentage(),
	)

	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Filename", Width: 20},
			{Title: "MIME", Width: 12},
			{Title: "Size", Width: 9},
		}),
		table.WithFocused(false),
	)

	return Model{
		assetService:      assetSvc,
		collectionService: colSvc,
		focusedPane:       focusSidebar,
		activeViewMode:    viewModeList,
		activeModal:       modalNone,
		showHistory:       true,
		table:             t,
		textInput:         ti,
		progress:          pg,
		previewCache:      make(map[string]string),
		cardPreviewCache:  make(map[string]string),
		history:           []string{fmt.Sprintf("[%s] System initialized", time.Now().Format("15:04:05"))},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadCollectionsCmd(),
		m.loadAssetsCmd(nil),
	)
}

// getActiveAssetIndex returns a strictly bounds-checked index
func (m Model) getActiveAssetIndex() int {
	if len(m.assets) == 0 {
		return 0
	}
	idx := m.table.Cursor()
	if m.activeViewMode == viewModeGrid {
		idx = m.gridCursor
	}
	if idx < 0 {
		return 0
	}
	if idx >= len(m.assets) {
		return len(m.assets) - 1
	}
	return idx
}

// clampCursors ensures both table and grid cursors remain valid
func (m Model) clampCursors() Model {
	n := len(m.assets)
	if n == 0 {
		m.gridCursor = 0
		m.table.SetCursor(0)
		return m
	}
	if m.gridCursor >= n {
		m.gridCursor = n - 1
	}
	if m.gridCursor < 0 {
		m.gridCursor = 0
	}
	if m.table.Cursor() >= n {
		m.table.SetCursor(n - 1)
	}
	if m.table.Cursor() < 0 {
		m.table.SetCursor(0)
	}
	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetHeight(msg.Height - 8)

	case collectionsLoadedMsg:
		m.collections = msg
		if m.selectedCollectionIdx > len(msg) {
			m.selectedCollectionIdx = len(msg)
		}

	case assetsLoadedMsg:
		m.assets = msg
		var rows []table.Row
		for _, a := range msg {
			rows = append(rows, table.Row{
				a.Filename,
				a.MimeType,
				fmt.Sprintf("%d B", a.ByteSize),
			})
		}
		m.table.SetRows(rows)
		m = m.clampCursors()

		if len(msg) == 0 {
			m.currentPreview = ""
		} else {
			curr := m.getActiveAssetIndex()
			cmds = append(cmds, m.loadPreviewCmd(msg[curr]))
			for _, a := range msg {
				cmds = append(cmds, m.loadCardPreviewCmd(a))
			}
		}

	case uploadProgressMsg:
		cmd = m.progress.SetPercent(float64(msg))
		return m, tea.Batch(cmd, m.listenUploadCmd())

	case uploadCompleteMsg:
		m.isUploading = false
		m.activeModal = modalNone
		if msg.err != nil {
			m = m.log(fmt.Sprintf("❌ Upload error: %v", msg.err))
			return m, nil
		}
		m = m.log(fmt.Sprintf("✅ Uploaded '%s'", msg.asset.Filename))
		return m, m.selectCurrentCollectionCmd()

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case previewLoadedMsg:
		m.previewCache[msg.assetID] = msg.preview
		currIdx := m.getActiveAssetIndex()
		if len(m.assets) > currIdx && m.assets[currIdx].ID == msg.assetID {
			m.currentPreview = msg.preview
		}

	case cardPreviewLoadedMsg:
		m.cardPreviewCache[msg.assetID] = msg.preview

	case zoomPreviewLoadedMsg:
		m.zoomPreview = string(msg)

	case tea.KeyMsg:
		// 1. Modals
		if m.activeModal != modalNone {
			switch m.activeModal {
			case modalZoomPreview:
				switch msg.String() {
				case "esc", "q", "space", "enter":
					m.activeModal = modalNone
					m.zoomPreview = ""
					return m, nil
				}
				return m, nil

			case modalConfirmDelete:
				switch msg.String() {
				case "y", "Y", "enter":
					if m.deleteTarget == deleteCollection {
						_ = m.collectionService.Delete(context.Background(), m.deleteTargetID)
						m = m.log(fmt.Sprintf("🗑️ Deleted collection '%s'", m.deleteTargetName))
						m.selectedCollectionIdx = 0
						m.activeModal = modalNone
						return m, tea.Batch(m.loadCollectionsCmd(), m.loadAssetsCmd(nil))
					} else {
						_ = m.assetService.Delete(context.Background(), m.deleteTargetID)
						delete(m.previewCache, m.deleteTargetID)
						delete(m.cardPreviewCache, m.deleteTargetID)
						m = m.log(fmt.Sprintf("🗑️ Deleted asset '%s'", m.deleteTargetName))
						m.activeModal = modalNone
						return m, m.selectCurrentCollectionCmd()
					}
				case "n", "N", "esc", "q":
					m.activeModal = modalNone
					return m, nil
				}
				return m, nil

			case modalAddToCollection:
				switch msg.String() {
				case "esc", "q":
					m.activeModal = modalNone
					return m, nil
				case "up", "k":
					if m.pickerSelected > 0 {
						m.pickerSelected--
					}
				case "down", "j":
					if m.pickerSelected < len(m.collections)-1 {
						m.pickerSelected++
					}
				case "enter":
					currIdx := m.getActiveAssetIndex()
					if len(m.collections) > 0 && len(m.assets) > currIdx {
						selectedAsset := m.assets[currIdx]
						targetCol := m.collections[m.pickerSelected]
						_ = m.assetService.AddToCollection(context.Background(), selectedAsset.ID, targetCol.ID)
						m = m.log(fmt.Sprintf("📁 Added '%s' to '%s'", selectedAsset.Filename, targetCol.Name))
						m.activeModal = modalNone
						return m, nil
					}
				}
				return m, nil

			default: // Text Input Modals
				switch msg.String() {
				case "esc":
					m.textInput.Reset()
					m.activeModal = modalNone
					return m, nil
				case "enter":
					val := strings.TrimSpace(m.textInput.Value())
					if val != "" {
						switch m.activeModal {
						case modalCreateCollection:
							_, _ = m.collectionService.Create(context.Background(), val, "")
							m = m.log(fmt.Sprintf("📁 Created collection '%s'", val))
							m.activeModal = modalNone
							m.textInput.Reset()
							return m, m.loadCollectionsCmd()

						case modalRenameCollection:
							_, _ = m.collectionService.Update(context.Background(), m.editingColID, val, "")
							m = m.log(fmt.Sprintf("✏️ Renamed collection to '%s'", val))
							m.activeModal = modalNone
							m.textInput.Reset()
							return m, m.loadCollectionsCmd()

						case modalAddTag:
							currIdx := m.getActiveAssetIndex()
							if len(m.assets) > currIdx {
								selectedAsset := m.assets[currIdx]
								_ = m.assetService.AddTag(context.Background(), selectedAsset.ID, val)
								m = m.log(fmt.Sprintf("🏷️ Tagged '%s' with #%s", selectedAsset.Filename, val))
							}
							m.activeModal = modalNone
							m.textInput.Reset()
							return m, m.selectCurrentCollectionCmd()

						case modalUpload:
							m.isUploading = true
							m = m.log(fmt.Sprintf("🚀 Uploading: %s", val))
							m.textInput.Reset()
							return m, m.startUploadCmd(val)
						}
					}
				}
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
		}

		// 2. Global Shortcuts
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "H":
			m.showHistory = !m.showHistory
			if len(m.assets) > 0 {
				cmds = append(cmds, m.loadPreviewCmd(m.assets[m.getActiveAssetIndex()]))
			}
			return m, tea.Batch(cmds...)

		case "u":
			m.activeModal = modalUpload
			m.textInput.Placeholder = "Local path or URL..."
			m.textInput.SetValue("")
			m.textInput.Focus()
			return m, textinput.Blink
		}

		// 3. Sidebar Actions
		if m.focusedPane == focusSidebar {
			switch msg.String() {
			case "right", "l", "tab":
				m.focusedPane = focusTable
				if m.activeViewMode == viewModeList {
					m.table.Focus()
				}
				return m, nil

			case "up", "k":
				if m.selectedCollectionIdx > 0 {
					m.selectedCollectionIdx--
					return m, m.selectCurrentCollectionCmd()
				}
			case "down", "j":
				if m.selectedCollectionIdx < len(m.collections) {
					m.selectedCollectionIdx++
					return m, m.selectCurrentCollectionCmd()
				}
			case "n":
				m.activeModal = modalCreateCollection
				m.textInput.Placeholder = "Collection name..."
				m.textInput.SetValue("")
				m.textInput.Focus()
				return m, textinput.Blink
			case "e":
				if m.selectedCollectionIdx > 0 {
					col := m.collections[m.selectedCollectionIdx-1]
					m.activeModal = modalRenameCollection
					m.editingColID = col.ID
					m.textInput.Placeholder = "New name..."
					m.textInput.SetValue(col.Name)
					m.textInput.Focus()
					return m, textinput.Blink
				}
			case "d":
				if m.selectedCollectionIdx > 0 {
					col := m.collections[m.selectedCollectionIdx-1]
					m.activeModal = modalConfirmDelete
					m.deleteTarget = deleteCollection
					m.deleteTargetID = col.ID
					m.deleteTargetName = col.Name
					return m, nil
				}
			}
		}

		// 4. Assets Pane (List & Grid)
		if m.focusedPane == focusTable {
			currIdx := m.getActiveAssetIndex()

			switch msg.String() {
			case "v":
				if m.activeViewMode == viewModeList {
					m.activeViewMode = viewModeGrid
					m.table.Blur()
				} else {
					m.activeViewMode = viewModeList
					m.table.Focus()
				}
				m.gridCursor = currIdx
				m.table.SetCursor(currIdx)
				return m, nil

			case "space", "p":
				if len(m.assets) > currIdx {
					m.activeModal = modalZoomPreview
					return m, m.loadZoomPreviewCmd(m.assets[currIdx])
				}

			case "o":
				if len(m.assets) > currIdx {
					go m.openInSystemViewer(m.assets[currIdx])
					m = m.log(fmt.Sprintf("👁️ Opened '%s' in OS viewer", m.assets[currIdx].Filename))
					return m, nil
				}

			case "t":
				if len(m.assets) > 0 {
					m.activeModal = modalAddTag
					m.textInput.Placeholder = "tag-name..."
					m.textInput.SetValue("")
					m.textInput.Focus()
					return m, textinput.Blink
				}

			case "a":
				if len(m.assets) > 0 && len(m.collections) > 0 {
					m.activeModal = modalAddToCollection
					m.pickerSelected = 0
					return m, nil
				}

			case "x":
				if m.selectedCollectionIdx > 0 && len(m.assets) > currIdx {
					selectedAsset := m.assets[currIdx]
					currentCol := m.collections[m.selectedCollectionIdx-1]
					_ = m.assetService.RemoveFromCollection(context.Background(), selectedAsset.ID, currentCol.ID)
					m = m.log(fmt.Sprintf("Removed '%s' from '%s'", selectedAsset.Filename, currentCol.Name))
					return m, m.selectCurrentCollectionCmd()
				}

			case "d":
				if len(m.assets) > currIdx {
					selectedAsset := m.assets[currIdx]
					m.activeModal = modalConfirmDelete
					m.deleteTarget = deleteAsset
					m.deleteTargetID = selectedAsset.ID
					m.deleteTargetName = selectedAsset.Filename
					return m, nil
				}
			}

			// Grid Movement
			if m.activeViewMode == viewModeGrid {
				cols := m.gridCols
				if cols <= 0 {
					cols = 2
				}

				switch msg.String() {
				case "left", "h":
					if m.gridCursor%cols == 0 {
						m.focusedPane = focusSidebar
						return m, nil
					}
					if m.gridCursor > 0 {
						m.gridCursor--
						m.table.SetCursor(m.gridCursor)
						return m, m.loadPreviewCmd(m.assets[m.gridCursor])
					}

				case "right", "l":
					if m.gridCursor < len(m.assets)-1 {
						m.gridCursor++
						m.table.SetCursor(m.gridCursor)
						return m, m.loadPreviewCmd(m.assets[m.gridCursor])
					}

				case "up", "k":
					if m.gridCursor-cols >= 0 {
						m.gridCursor -= cols
						m.table.SetCursor(m.gridCursor)
						return m, m.loadPreviewCmd(m.assets[m.gridCursor])
					}

				case "down", "j":
					if m.gridCursor+cols < len(m.assets) {
						m.gridCursor += cols
						m.table.SetCursor(m.gridCursor)
						return m, m.loadPreviewCmd(m.assets[m.gridCursor])
					}
				}
				return m, nil
			}

			// List Movement
			if m.activeViewMode == viewModeList {
				switch msg.String() {
				case "left", "h":
					m.focusedPane = focusSidebar
					m.table.Blur()
					return m, nil
				}

				prevCursor := m.table.Cursor()
				m.table, cmd = m.table.Update(msg)
				cmds = append(cmds, cmd)

				newCursor := m.table.Cursor()
				if newCursor != prevCursor && len(m.assets) > newCursor {
					m.gridCursor = newCursor
					cmds = append(cmds, m.loadPreviewCmd(m.assets[newCursor]))
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.activeModal != modalNone {
		return m.renderModalOverlay()
	}

	sidebarWidth := 18
	inspectorWidth := 46
	mainWidth := m.width - sidebarWidth - inspectorWidth - 8
	if mainWidth < 25 {
		mainWidth = 25
	}

	totalRightHeight := m.height - 4

	// 1. Sidebar
	var sidebarContent string
	sidebarContent += titleStyle.Render("📁 Collections") + "\n\n"
	if m.selectedCollectionIdx == 0 {
		sidebarContent += selectedItemStyle.Render("▸ [ All Assets ]") + "\n"
	} else {
		sidebarContent += "  [ All Assets ]\n"
	}
	for i, c := range m.collections {
		idx := i + 1
		if m.selectedCollectionIdx == idx {
			sidebarContent += selectedItemStyle.Render(fmt.Sprintf("▸ %s", c.Name)) + "\n"
		} else {
			sidebarContent += fmt.Sprintf("  %s\n", c.Name)
		}
	}

	sidebarStyle := inactiveBox
	if m.focusedPane == focusSidebar {
		sidebarStyle = activeBox
	}
	renderedSidebar := sidebarStyle.Width(sidebarWidth).Height(totalRightHeight).Render(sidebarContent)

	// 2. Main Pane (Center)
	mainBoxStyle := inactiveBox
	if m.focusedPane == focusTable {
		mainBoxStyle = activeBox
	}

	var renderedMain string
	if m.activeViewMode == viewModeList {
		header := titleStyle.Render("📦 Assets (List View)") + "\n\n"
		renderedMain = mainBoxStyle.Width(mainWidth).Height(totalRightHeight).Render(header + m.table.View())
	} else {
		renderedMain = mainBoxStyle.Width(mainWidth).Height(totalRightHeight).Render(m.renderGridView(mainWidth, totalRightHeight-2))
	}

	// 3. Inspector Content (Top Right)
	var inspectorContent string
	currIdx := m.getActiveAssetIndex()

	if len(m.assets) > 0 && currIdx >= 0 && currIdx < len(m.assets) {
		a := m.assets[currIdx]
		inspectorContent += titleStyle.Render("ℹ️ Inspector") + "\n\n"

		if preview, ok := m.previewCache[a.ID]; ok && preview != "" {
			inspectorContent += preview + "\n"
		} else if m.currentPreview != "" {
			inspectorContent += m.currentPreview + "\n"
		}

		inspectorContent += fmt.Sprintf("Name: %s\n", a.Filename)
		inspectorContent += fmt.Sprintf("Size: %d bytes\n", a.ByteSize)

		if a.Metadata.Width != nil && a.Metadata.Height != nil {
			inspectorContent += fmt.Sprintf("Dims: %d × %d px\n", *a.Metadata.Width, *a.Metadata.Height)
		}
		if a.Metadata.PageCount != nil {
			inspectorContent += fmt.Sprintf("Pages: %d\n", *a.Metadata.PageCount)
		}

		if len(a.Tags) > 0 {
			var tagNames []string
			for _, t := range a.Tags {
				tagNames = append(tagNames, "#"+t.Name)
			}
			inspectorContent += "Tags: " + strings.Join(tagNames, " ") + "\n"
		}
	} else {
		inspectorContent += titleStyle.Render("ℹ️ Inspector") + "\n\nNo asset selected\n"
	}

	// 4. Right Column Layout
	var renderedRightColumn string

	if m.showHistory {
		topHeight := totalRightHeight / 2
		bottomHeight := totalRightHeight - topHeight - 2

		renderedInspector := inactiveBox.Width(inspectorWidth).Height(topHeight).Render(inspectorContent)

		var historyContent string
		historyContent += titleStyle.Render("📜 Activity History") + " " + helpStyle.Render("(H: Hide)") + "\n\n"

		startLog := 0
		maxLogs := (bottomHeight - 4)
		if maxLogs < 1 {
			maxLogs = 1
		}
		if len(m.history) > maxLogs {
			startLog = len(m.history) - maxLogs
		}
		for _, l := range m.history[startLog:] {
			historyContent += helpStyle.Render(l) + "\n"
		}

		renderedHistory := inactiveBox.Width(inspectorWidth).Height(bottomHeight).Render(historyContent)
		renderedRightColumn = lipgloss.JoinVertical(lipgloss.Left, renderedInspector, renderedHistory)
	} else {
		renderedRightColumn = inactiveBox.Width(inspectorWidth).Height(totalRightHeight).Render(inspectorContent)
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, renderedSidebar, renderedMain, renderedRightColumn)

	var helpText string
	if m.focusedPane == focusSidebar {
		helpText = "←/→: Switch • ↑/↓: Navigate • n: New • e: Rename • d: Delete • H: History • q: Quit"
	} else if m.focusedPane == focusTable {
		helpText = "v: View Mode • Space/p: Zoom • o: Open • a: Col • t: Tag • d: Del • H: History • u: Upload"
	}

	return lipgloss.JoinVertical(lipgloss.Left, body, helpStyle.Render(helpText))
}

func (m Model) renderGridView(width, height int) string {
	if len(m.assets) == 0 {
		return titleStyle.Render("📦 Assets (Grid View)") + "\n\n" + helpStyle.Render("No assets in this collection.")
	}

	minCardW := 18
	cols := width / (minCardW + 2)
	if cols < 1 {
		cols = 1
	}
	if cols > 4 {
		cols = 4
	}

	cardW := (width / cols) - 2
	if cardW < minCardW {
		cardW = minCardW
	}

	var cards []string
	for i, a := range m.assets {
		cardStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtleColor).
			Padding(0, 1).
			Width(cardW)

		if i == m.gridCursor {
			cardStyle = cardStyle.BorderForeground(activeColor).Bold(true)
		}

		previewStr := "  [ No Preview ]  "
		if p, ok := m.cardPreviewCache[a.ID]; ok && p != "" {
			previewStr = p
		}

		dispName := a.Filename
		maxNameLen := cardW - 4
		if len(dispName) > maxNameLen {
			dispName = dispName[:maxNameLen-3] + "..."
		}

		cardBody := fmt.Sprintf("%s\n%s\n%s", previewStr, dispName, helpStyle.Render(fmt.Sprintf("%d B", a.ByteSize)))
		cards = append(cards, cardStyle.Render(cardBody))
	}

	var rows []string
	for i := 0; i < len(cards); i += cols {
		end := i + cols
		if end > len(cards) {
			end = len(cards)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...))
	}

	cursorRow := m.gridCursor / cols
	visibleRowsCount := (height - 4) / 7
	if visibleRowsCount < 1 {
		visibleRowsCount = 1
	}

	startRow := 0
	if cursorRow >= visibleRowsCount {
		startRow = cursorRow - visibleRowsCount + 1
	}
	endRow := startRow + visibleRowsCount
	if endRow > len(rows) {
		endRow = len(rows)
	}

	gridContent := strings.Join(rows[startRow:endRow], "\n")
	header := titleStyle.Render(fmt.Sprintf("📦 Assets (Grid View - %d items)", len(m.assets))) + "\n\n"
	return header + gridContent
}

func (m Model) renderModalOverlay() string {
	if m.activeModal == modalZoomPreview {
		zoomBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

		content := titleStyle.Render("🔍 Fullscreen Asset Preview") + "\n\n"
		if m.zoomPreview != "" {
			content += m.zoomPreview + "\n"
		} else {
			content += "Loading high-resolution preview...\n"
		}
		content += helpStyle.Render("Space / Esc: Close Zoom Preview")

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, zoomBox.Render(content))
	}

	modalBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Width(48)

	var content string

	switch m.activeModal {
	case modalConfirmDelete:
		targetDesc := "collection"
		if m.deleteTarget == deleteAsset {
			targetDesc = "asset"
		}
		content = titleStyle.Render(fmt.Sprintf("⚠️ Delete %s?", targetDesc)) + "\n\n"
		content += fmt.Sprintf("Are you sure you want to delete '%s'?\n\n", m.deleteTargetName)
		content += selectedItemStyle.Render("[ y / Enter ] Confirm") + "    " + helpStyle.Render("[ n / Esc ] Cancel")

	case modalAddToCollection:
		content = titleStyle.Render("📁 Add Asset to Collection") + "\n\n"
		for i, c := range m.collections {
			if i == m.pickerSelected {
				content += selectedItemStyle.Render(fmt.Sprintf("▸ %s\n", c.Name))
			} else {
				content += fmt.Sprintf("  %s\n", c.Name)
			}
		}
		content += "\n" + helpStyle.Render("↑/↓: Select • Enter: Confirm • Esc: Cancel")

	case modalUpload:
		if m.isUploading {
			content = titleStyle.Render("🚀 Uploading Asset...") + "\n\n"
			content += m.progress.View() + "\n\n"
			content += helpStyle.Render("Streaming and extracting metadata...")
		} else {
			content = titleStyle.Render("🚀 Upload Asset") + "\n\n"
			content += "Enter Local Path or HTTP(S) URL:\n\n"
			content += m.textInput.View() + "\n\n"
			content += helpStyle.Render("Enter: Start Upload • Esc: Cancel")
		}

	case modalCreateCollection:
		content = titleStyle.Render("📁 New Collection") + "\n\n"
		content += "Collection Name:\n\n"
		content += m.textInput.View() + "\n\n"
		content += helpStyle.Render("Enter: Create • Esc: Cancel")

	case modalRenameCollection:
		content = titleStyle.Render("✏️ Rename Collection") + "\n\n"
		content += "New Name:\n\n"
		content += m.textInput.View() + "\n\n"
		content += helpStyle.Render("Enter: Save • Esc: Cancel")

	case modalAddTag:
		content = titleStyle.Render("🏷️ Add Tag") + "\n\n"
		content += "Tag Name:\n\n"
		content += m.textInput.View() + "\n\n"
		content += helpStyle.Render("Enter: Add • Esc: Cancel")
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalBox.Render(content))
}

func (m Model) log(message string) Model {
	entry := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), message)
	m.history = append(m.history, entry)
	return m
}

func (m Model) openInSystemViewer(asset domain.Asset) {
	path := fmt.Sprintf("./data/storage/%s", asset.StorageKey)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	}
	if cmd != nil {
		_ = cmd.Run()
	}
}

// Commands
func (m Model) loadCollectionsCmd() tea.Cmd {
	return func() tea.Msg {
		cols, err := m.collectionService.List(context.Background())
		if err != nil {
			return errMsg(err)
		}
		return collectionsLoadedMsg(cols)
	}
}

func (m Model) loadAssetsCmd(colID *string) tea.Cmd {
	return func() tea.Msg {
		assets, err := m.assetService.List(context.Background(), colID)
		if err != nil {
			return errMsg(err)
		}
		return assetsLoadedMsg(assets)
	}
}

func (m Model) selectCurrentCollectionCmd() tea.Cmd {
	if m.selectedCollectionIdx == 0 {
		return m.loadAssetsCmd(nil)
	}
	col := m.collections[m.selectedCollectionIdx-1]
	return m.loadAssetsCmd(&col.ID)
}

func (m Model) startUploadCmd(source string) tea.Cmd {
	progChan := make(chan float64, 100)
	doneChan := make(chan uploadCompleteMsg, 1)

	go func() {
		asset, err := m.assetService.UploadFromSource(context.Background(), source, func(percent float64) {
			select {
			case progChan <- percent:
			default:
			}
		})
		doneChan <- uploadCompleteMsg{asset: asset, err: err}
	}()

	return m.listenUploadChansCmd(progChan, doneChan)
}

func (m Model) listenUploadChansCmd(progChan chan float64, doneChan chan uploadCompleteMsg) tea.Cmd {
	return func() tea.Msg {
		select {
		case res := <-doneChan:
			return res
		case p := <-progChan:
			return uploadProgressMsg(p)
		}
	}
}

func (m Model) listenUploadCmd() tea.Cmd {
	return func() tea.Msg {
		if m.doneChan == nil || m.progressChan == nil {
			return nil
		}
		select {
		case res := <-m.doneChan:
			return res
		case p := <-m.progressChan:
			return uploadProgressMsg(p)
		}
	}
}

func (m Model) loadPreviewCmd(asset domain.Asset) tea.Cmd {
	return func() tea.Msg {
		if preview, ok := m.previewCache[asset.ID]; ok {
			return previewLoadedMsg{assetID: asset.ID, preview: preview}
		}

		rc, err := m.assetService.OpenPreview(context.Background(), asset.ID)
		if err != nil {
			rc, err = m.assetService.Open(context.Background(), asset.ID)
			if err != nil {
				return previewLoadedMsg{assetID: asset.ID, preview: ""}
			}
		}
		defer rc.Close()

		targetCols := 42
		maxRows := 12
		if !m.showHistory && m.height > 30 {
			maxRows = m.height - 14
		}

		preview, err := RenderPreview(rc, targetCols, maxRows)
		if err != nil {
			return previewLoadedMsg{assetID: asset.ID, preview: ""}
		}
		return previewLoadedMsg{assetID: asset.ID, preview: preview}
	}
}

func (m Model) loadCardPreviewCmd(asset domain.Asset) tea.Cmd {
	return func() tea.Msg {
		if preview, ok := m.cardPreviewCache[asset.ID]; ok {
			return cardPreviewLoadedMsg{assetID: asset.ID, preview: preview}
		}

		rc, err := m.assetService.OpenPreview(context.Background(), asset.ID)
		if err != nil {
			rc, err = m.assetService.Open(context.Background(), asset.ID)
			if err != nil {
				return cardPreviewLoadedMsg{assetID: asset.ID, preview: ""}
			}
		}
		defer rc.Close()

		preview, err := RenderPreview(rc, 14, 3)
		if err != nil {
			return cardPreviewLoadedMsg{assetID: asset.ID, preview: ""}
		}
		return cardPreviewLoadedMsg{assetID: asset.ID, preview: preview}
	}
}

func (m Model) loadZoomPreviewCmd(asset domain.Asset) tea.Cmd {
	return func() tea.Msg {
		rc, err := m.assetService.OpenPreview(context.Background(), asset.ID)
		if err != nil {
			rc, err = m.assetService.Open(context.Background(), asset.ID)
			if err != nil {
				return zoomPreviewLoadedMsg("")
			}
		}
		defer rc.Close()

		zoomCols := m.width - 12
		zoomRows := m.height - 8

		preview, err := RenderPreview(rc, zoomCols, zoomRows)
		if err != nil {
			return zoomPreviewLoadedMsg("")
		}
		return zoomPreviewLoadedMsg(preview)
	}
}
