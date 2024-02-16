package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fasthttp/websocket"
)

type Message struct {
	RoleName    string `json:"role"`
	RoleColor   string `json:"roleColor"`
	SenderName  string `json:"sender"`
	SenderColor string `json:"userColor"`
	Content     []byte `json:"content"`
}

func (cm *Message) Format() string {
	usernameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cm.SenderColor))

	roleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color(cm.RoleColor)).
		PaddingRight(1).
		PaddingLeft(1).
		MarginRight(1)

	if cm.RoleName == "" {
		return usernameStyle.Render(cm.SenderName) + ": " + string(cm.Content)
	}

	return roleStyle.Render(cm.RoleName) + usernameStyle.Render(cm.SenderName) + ": " + string(cm.Content)
}

func generatePaleColorHex() string {
	src := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(src)

	r := rng.Intn(156) + 100 // 100 to 255 range
	g := rng.Intn(156) + 100 // 100 to 255 range
	b := rng.Intn(156) + 100 // 100 to 255 range

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// Define styles
type Styles struct {
	BorderColor lipgloss.Color
	InputField  lipgloss.Style
}

type User struct {
	conn *websocket.Conn
}

func DefaultStyles() *Styles {
	s := new(Styles)
	s.BorderColor = lipgloss.Color("#3C3C3C")
	s.InputField = lipgloss.NewStyle().Padding(1).Width(80)
	return s
}

// Main application model
type Main struct {
	styles        *Styles
	input         textinput.Model
	messages      []Message
	width         int
	height        int
	scrollPos     int
	user          *websocket.Conn
	program       *tea.Program
	username      string
	usernameColor string
	role          string
	roleColor     string
}

// Constructor for Main
func New() *Main {
	styles := DefaultStyles()
	input := textinput.New()
	input.Placeholder = "Type your message..."
	input.Focus()
	input.CharLimit = 156
	return &Main{styles: styles, input: input, messages: []Message{}}
}

// Initialize the application
func (m Main) Init() tea.Cmd {
	return nil
}

func (m *Main) checkCommand(command string) {
	splitCommand := strings.Split(command, " ")
	commandName := splitCommand[0]
	var commandArg string

	if len(splitCommand) > 1 {
		commandArg = splitCommand[1]
	}

	switch commandName {
	case "nick":
		m.username = commandArg
	case "role":
		role := strings.ToUpper(commandArg)
		m.role = role
	case "color":
		m.usernameColor = commandArg
		m.roleColor = commandArg
	case "nickcolor":
		m.usernameColor = commandArg
	case "rolecolor":
		m.roleColor = commandArg
	case "randomcolor":
		randomColor := generatePaleColorHex()
		m.usernameColor = randomColor
		m.roleColor = randomColor
	}
}

// Update function to handle user input and terminal resize events
func (m Main) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.input.Value() != "" {
				if m.input.Value()[0] == '/' {
					m.checkCommand(m.input.Value()[1:])
				} else {
					newMessage := Message{
						RoleName:    m.role,
						RoleColor:   m.roleColor,
						SenderName:  m.username,
						SenderColor: m.usernameColor,
						Content:     []byte(m.input.Value()),
					}
					msgByte, err := json.Marshal(newMessage)
					if err != nil {
						log.Fatalf("ERROR JSON: %v", err)
						return m, nil
					}
					m.user.WriteMessage(websocket.TextMessage, msgByte)
				}
				m.input.SetValue("")
			}
		case tea.KeyUp:
			if m.scrollPos > 0 {
				m.scrollPos--
			}
		case tea.KeyDown:
			if m.scrollPos < len(m.messages)-1 {
				m.scrollPos++
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = m.width - 7
	case Message:
		m.messages = append(m.messages, msg)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View function to render the UI
func (m Main) View() string {
	chatHeight := m.height - 2
	// visibleMessages := m.messages
	// if len(visibleMessages) > chatHeight && chatHeight > 0 {
	// 	start := m.scrollPos
	// 	end := start + chatHeight
	// 	if end > len(visibleMessages) {
	// 		start = len(visibleMessages) - chatHeight
	// 		end = len(visibleMessages)
	// 	}
	//
	// 	visibleMessages = visibleMessages[start:end]
	// }

	chatViewStyle := lipgloss.NewStyle().
		Height(chatHeight).
		Width(m.width - 2).
		PaddingLeft(1).
		PaddingRight(1)

	var sb strings.Builder
	for i, msg := range m.messages {
		sb.WriteString(msg.Format())
		if i < len(m.messages)-1 {
			sb.WriteString("\n")
		}
	}
	combinedMessages := sb.String()

	chatView := chatViewStyle.Render(combinedMessages)

	inputView := m.styles.InputField.Copy().
		Height(1).
		Width(m.width-2).
		Padding(0, 1, 0, 1).
		Render(m.input.View())

	return lipgloss.JoinVertical(lipgloss.Top, chatView, inputView)
}

func main() {
	main := New()

	wsURL := "ws://localhost:5173/ws"

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatalf("Error connecting to WS: %v", err)
	}
	defer conn.Close()
	randomColor := generatePaleColorHex()
	main.user = conn
	main.username = "Anonymous"
	main.usernameColor = randomColor
	main.role = ""
	main.roleColor = randomColor

	// Function to handle incoming messages
	go func() {
		for {
			// Read message type and message from the WebSocket
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}

			var chatMessage Message
			err = json.Unmarshal(message, &chatMessage)
			if err != nil {
				log.Printf("ERROR: %v", err)
				return
			}

			main.program.Send(chatMessage)
		}
	}()

	p := tea.NewProgram(main, tea.WithAltScreen())
	main.program = p
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %s\n", err)
		os.Exit(1)
	}
}
