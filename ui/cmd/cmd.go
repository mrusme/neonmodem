package cmd

import tea "github.com/charmbracelet/bubbletea"

type CallType int8

const (
	WinOpen CallType = iota
	WinClose
	WinFocus
	WinBlur
	WinRefreshData
	WinFreshData

	ViewFocus
	ViewBlur
	ViewRefreshData
	ViewFreshData

	MsgError
)

type Arg struct {
	Name  string
	Value interface{}
}

type Command struct {
	Call   CallType
	Target string
	Args   map[string]interface{}
}

func New(
	call CallType,
	target string,
	args ...Arg,
) *Command {
	cmd := new(Command)
	cmd.Call = call
	cmd.Target = target
	cmd.Args = make(map[string]interface{})

	for _, arg := range args {
		cmd.Args[arg.Name] = arg.Value
	}

	return cmd
}

func (cmd *Command) Tea() tea.Cmd {
	return func() tea.Msg {
		return *cmd
	}
}

func (cmd *Command) AddArg(name string, value interface{}) {
	cmd.Args[name] = value
}

func (cmd *Command) GetArg(name string) interface{} {
	if iface, ok := cmd.Args[name]; ok {
		return iface
	}

	return nil
}

func (cmd *Command) GetArgs() []Arg {
	var args []Arg

	for name, value := range cmd.Args {
		args = append(args, Arg{Name: name, Value: value})
	}

	return args
}
