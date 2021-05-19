package layout

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"

	"github.com/Luismorlan/btc_in_go/commands"
	"github.com/jroimartin/gocui"
)

type cmd struct {
	str   string
	ready bool
	m     sync.RWMutex
}

var command cmd = cmd{}

// PastCmd is the ViewManager that logs past command.
type PastCmd struct {
	name string
}

// Input box for command.
type FullNodeInput struct {
	name string
	cmd  chan commands.Command
}

type WalletInput struct {
	name string
	cmd  chan commands.ClientCommand
}

type Logger struct {
	name string
}

type Manual struct {
	name string
	path string
}

func (pc *PastCmd) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	// Bottom left corner.
	v, _ := g.SetView(pc.name, 1, maxY*2/3, maxX/3, maxY-6)
	v.Autoscroll = true
	v.Wrap = true

	command.m.RLock()
	defer command.m.RUnlock()
	if command.ready {
		fmt.Fprintln(v, "> "+command.str)
	}
	command.ready = false

	return nil
}

func (i *FullNodeInput) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	// Bottom left.
	v, err := g.SetView(i.name, 1, maxY-5, maxX-1, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}
	v.Wrap = true
	v.Autoscroll = true
	v.Editor = i
	v.Editable = true
	return nil
}

func (w *WalletInput) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	// Bottom left.
	v, err := g.SetView(w.name, 1, maxY-5, maxX-1, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}
	v.Wrap = true
	v.Autoscroll = true
	v.Editor = w
	v.Editable = true
	return nil
}

func (l *Logger) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	// Bottom left corner.
	v, _ := g.SetView(l.name, maxX/3+1, 1, maxX-1, maxY-6)
	v.Autoscroll = true
	v.Wrap = true
	return nil
}

func (m *Manual) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	// Bottom left corner.
	v, _ := g.SetView(m.name, 1, 1, maxX/3, maxY*2/3-1)
	v.Autoscroll = true
	v.Wrap = true
	v.Clear()
	dat, err := ioutil.ReadFile(m.path)
	if err != nil {
		g.Close()
		log.Fatal(err)
	}
	fmt.Fprintln(v, string(dat))
	return nil
}

func (i *FullNodeInput) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case key == gocui.KeyEnter:
		// Read buffer.
		s := v.Buffer()
		// Remove \n from string.
		s = strings.Replace(s, "\n", "", -1)
		op, err := commands.CreateCommand(s)
		command.m.Lock()
		command.str = s
		if err != nil {
			command.str = s + "\n" + err.Error()
		}
		command.ready = true
		command.m.Unlock()
		if err == nil {
			// If a valid command, send to fullnode for processing.
			i.cmd <- op
		}

		// Reset cursor.
		v.Clear()
		v.SetOrigin(0, 0)
		v.SetCursor(0, 0)

	case ch != 0 && mod == 0:
		v.EditWrite(ch)
	case key == gocui.KeySpace:
		v.EditWrite(' ')
	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		v.EditDelete(true)
	}
}

func (w *WalletInput) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case key == gocui.KeyEnter:
		// Read buffer.
		s := v.Buffer()
		// Remove \n from string.
		s = strings.Replace(s, "\n", "", -1)
		op, err := commands.CreateClientCommand(s)
		command.m.Lock()
		command.str = s
		if err != nil {
			command.str = s + "\n" + err.Error()
		}
		command.ready = true
		command.m.Unlock()
		if err == nil {
			// If a valid command, send to fullnode for processing.
			w.cmd <- op
		}

		// Reset cursor.
		v.Clear()
		v.SetOrigin(0, 0)
		v.SetCursor(0, 0)

	case ch != 0 && mod == 0:
		v.EditWrite(ch)
	case key == gocui.KeySpace:
		v.EditWrite(' ')
	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		v.EditDelete(true)
	}
}

func SetFocus(name string) func(g *gocui.Gui) error {
	return func(g *gocui.Gui) error {
		_, err := g.SetCurrentView(name)
		return err
	}
}

// Create a GUI, using the command channel to pass command to fullnode.
func CreateGui(cmd interface{}, manual_path string) (*gocui.Gui, error) {
	is_full_node := true
	switch cmd.(type) {
	case chan commands.Command:
		is_full_node = true
	case chan commands.ClientCommand:
		is_full_node = false
	default:
		log.Fatalln("invalid command channel")
	}

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return nil, err
	}

	g.Cursor = true

	pc := &PastCmd{name: "pastcommand"}
	l := &Logger{name: "logger"}
	m := &Manual{name: "manual", path: manual_path}
	focus := gocui.ManagerFunc(SetFocus("input"))
	if is_full_node {
		input := &FullNodeInput{name: "input", cmd: cmd.(chan commands.Command)}
		g.SetManager(pc, input, l, m, focus)
	} else {
		input := &WalletInput{name: "input", cmd: cmd.(chan commands.ClientCommand)}
		g.SetManager(pc, input, l, m, focus)
	}

	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Panicln(err)
	}

	return g, err
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
