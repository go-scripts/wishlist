package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/go-scripts/bubbles/spinner"
	tea "github.com/go-scripts/bubbletea"
	"github.com/go-scripts/wish"
	"github.com/go-scripts/wish/activeterm"
	bm "github.com/go-scripts/wish/bubbletea"
	"github.com/go-scripts/wish/logging"
	lm "github.com/go-scripts/wish/logging"
	"github.com/go-scripts/wishlist"
	gossh "golang.org/x/crypto/ssh"
)

func main() {
	k, err := keygen.New(
		filepath.Join(".ssh_keys", "server"),
		keygen.WithKeyType(keygen.Ed25519),
	)
	if err != nil {
		log.Fatal("Server keypair", "err", err)
	}
	if !k.KeyPairExists() {
		if err := k.WriteKeys(); err != nil {
			log.Fatal("Server keypair", "err", err)
		}
	}

	// wishlist config
	cfg := &wishlist.Config{
		Title: "SSH Directories",
		Port:  2233,
		Factory: func(e wishlist.Endpoint) (*ssh.Server, error) {
			if e.IsMainServer {
				return wish.NewServer(
					wish.WithAddress(e.Address),
					wish.WithHostKeyPEM(k.RawPrivateKey()),
					wish.WithKeyboardInteractiveAuth(func(_ ssh.Context, challenger gossh.KeyboardInteractiveChallenge) bool {
						log.Info("keyboard-interactive")
						answers, err := challenger(
							"", "",
							[]string{
								"♦ How much is 2+3: ",
								"♦ Which editor is best, vim or emacs? ",
							},
							[]bool{true, true},
						)
						if err != nil {
							return false
						}
						// here we check for the correct answers:
						return len(answers) == 2 && answers[0] == "5" && answers[1] == "vim"
					}),
					wish.WithMiddleware(
						append(
							e.Middlewares, // this is the important bit: the middlewares from the endpoint
							lm.Middleware(),
							activeterm.Middleware(),
						)...,
					),
				)
			}

			return wish.NewServer(
				wish.WithAddress(e.Address),
				wish.WithHostKeyPEM(k.RawPrivateKey()),
				wish.WithMiddleware(
					append(
						e.Middlewares, // this is the important bit: the middlewares from the endpoint
						lm.Middleware(),
						activeterm.Middleware(),
					)...,
				),
			)
		},
		Endpoints: []*wishlist.Endpoint{
			{
				Name: "bubbletea",
				Middlewares: []wish.Middleware{
					logging.Middleware(),
					bm.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
						r := bm.MakeRenderer(s)
						return initialModel(r), nil
					}),
				},
			},
			{
				Name: "simple",
				Middlewares: []wish.Middleware{
					func(h ssh.Handler) ssh.Handler {
						return func(s ssh.Session) {
							_, _ = s.Write([]byte("hello, world\n\r"))
							time.Sleep(time.Second * 5)
							h(s)
						}
					},
				},
			},
			{
				Name:    "app2",
				Address: "app.addr:2222",
			},
			{
				Name:    "server1",
				Address: "server1:22",
			},
			{
				Name:    "server2",
				Address: "server1:22",
				User:    "override_user",
			},
			{
				Name: "entries without middlewares and addresses are ignored",
			},
			{
				Address: "entries without names are ignored",
			},
		},
	}

	// start all the servers
	if err := wishlist.Serve(cfg); err != nil {
		log.Fatal("Serve", "err", err)
	}
}

type model struct {
	spinner  spinner.Model
	renderer *lipgloss.Renderer
}

func initialModel(r *lipgloss.Renderer) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return model{spinner: s, renderer: r}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Info("keypress", "msg", msg)
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		log.Info("window size", "msg", msg)
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m model) View() string {
	str := fmt.Sprintf(
		"\n\n   %s Loading forever... press q to quit\n\n",
		m.renderer.NewStyle().Foreground(lipgloss.Color("5")).Render(m.spinner.View()),
	)
	return str
}
