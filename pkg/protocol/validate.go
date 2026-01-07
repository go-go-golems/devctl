package protocol

import "github.com/pkg/errors"

func ValidateHandshake(h Handshake) error {
	if h.Type != FrameHandshake {
		return errors.Errorf("%s: expected handshake frame, got %q", ErrProtocolInvalidHandshake, h.Type)
	}
	if h.ProtocolVersion != ProtocolV2 {
		return errors.Errorf("%s: unsupported protocol_version %q", ErrProtocolInvalidHandshake, h.ProtocolVersion)
	}
	if h.PluginName == "" {
		return errors.Errorf("%s: missing plugin_name", ErrProtocolInvalidHandshake)
	}
	seenCommands := map[string]struct{}{}
	for i, cmd := range h.Capabilities.Commands {
		if cmd.Name == "" {
			return errors.Errorf("%s: capabilities.commands[%d] missing name", ErrProtocolInvalidHandshake, i)
		}
		if _, ok := seenCommands[cmd.Name]; ok {
			return errors.Errorf("%s: duplicate command name %q", ErrProtocolInvalidHandshake, cmd.Name)
		}
		seenCommands[cmd.Name] = struct{}{}
		for j, arg := range cmd.ArgsSpec {
			if arg.Name == "" {
				return errors.Errorf("%s: capabilities.commands[%d].args_spec[%d] missing name", ErrProtocolInvalidHandshake, i, j)
			}
			if arg.Type == "" {
				return errors.Errorf("%s: capabilities.commands[%d].args_spec[%d] missing type", ErrProtocolInvalidHandshake, i, j)
			}
		}
	}
	return nil
}
