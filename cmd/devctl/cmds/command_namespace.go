package cmds

import (
	"fmt"

	"github.com/spf13/cobra"
)

// CommandNamespace is the authoritative set of command names and aliases in a
// Cobra command's immediate child namespace. It keeps collision policy
// independent from Cobra rendering and avoids manually duplicating built-ins.
type CommandNamespace struct {
	names map[string]struct{}
}

func NewCommandNamespace(reserved ...string) *CommandNamespace {
	namespace := &CommandNamespace{names: make(map[string]struct{}, len(reserved))}
	for _, name := range reserved {
		if name != "" {
			namespace.names[name] = struct{}{}
		}
	}
	return namespace
}

// RootCommandNamespace discovers the actual root children and reserves Cobra's
// implicit help/completion names, which may be materialized only during execute.
func RootCommandNamespace(root *cobra.Command) *CommandNamespace {
	namespace := NewCommandNamespace("help", "completion")
	for _, command := range root.Commands() {
		namespace.Reserve(command)
	}
	return namespace
}

// Add registers a command and aliases in the namespace before attaching it to
// the Cobra parent, rejecting collisions at the shared policy boundary.
func (n *CommandNamespace) Add(parent, command *cobra.Command) error {
	if parent == nil || command == nil {
		return fmt.Errorf("command namespace requires non-nil parent and command")
	}
	for _, name := range commandNames(command) {
		if n.Contains(name) {
			return fmt.Errorf("command namespace collision: %q is already reserved", name)
		}
	}
	n.Reserve(command)
	parent.AddCommand(command)
	return nil
}

func (n *CommandNamespace) Reserve(command *cobra.Command) {
	if n == nil || command == nil {
		return
	}
	for _, name := range commandNames(command) {
		n.names[name] = struct{}{}
	}
}

func (n *CommandNamespace) Contains(name string) bool {
	if n == nil {
		return false
	}
	_, exists := n.names[name]
	return exists
}

// Snapshot returns the catalog API's map representation without exposing
// mutable namespace state.
func (n *CommandNamespace) Snapshot() map[string]bool {
	reserved := make(map[string]bool, len(n.names))
	for name := range n.names {
		reserved[name] = true
	}
	return reserved
}

func commandNames(command *cobra.Command) []string {
	names := make([]string, 0, 1+len(command.Aliases))
	if command.Name() != "" {
		names = append(names, command.Name())
	}
	for _, alias := range command.Aliases {
		if alias != "" {
			names = append(names, alias)
		}
	}
	return names
}
