package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-go-golems/devctl/pkg/protocol"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigFilename   = ".devctl.yaml"
	DefaultOverrideFilename = ".devctl.override.yaml"
)

type File struct {
	Profile    ProfileBlock        `yaml:"profile,omitempty"`
	Profiles   map[string]*Profile `yaml:"profiles,omitempty"`
	Plugins    []Plugin            `yaml:"plugins"`
	Strictness string              `yaml:"strictness,omitempty"` // "warn" | "error"
}

type ProfileBlock struct {
	Active string `yaml:"active,omitempty"`
}

type Profile struct {
	DisplayName string            `yaml:"display_name,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Plugins     []string          `yaml:"plugins,omitempty"`
	Env         map[string]string `yaml:"env,omitempty"`
}

type Plugin struct {
	ID       string                 `yaml:"id"`
	Path     string                 `yaml:"path"`
	Args     []string               `yaml:"args,omitempty"`
	Priority int                    `yaml:"priority,omitempty"`
	WorkDir  string                 `yaml:"workdir,omitempty"`
	Env      map[string]string      `yaml:"env,omitempty"`
	Commands []protocol.CommandSpec `yaml:"commands,omitempty"`
}

func DefaultPath(repoRoot string) string {
	return filepath.Join(repoRoot, DefaultConfigFilename)
}

func DefaultOverridePath(repoRoot string) string {
	return filepath.Join(repoRoot, DefaultOverrideFilename)
}

func LoadFromFile(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "read config")
	}
	var cfg File
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, errors.Wrap(err, "parse config yaml")
	}
	return &cfg, nil
}

func LoadOptional(path string) (*File, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{}, nil
		}
		return nil, errors.Wrap(err, "stat config")
	}
	return LoadFromFile(path)
}

func LoadStacked(basePath, overridePath string) (*File, error) {
	base, err := LoadOptional(basePath)
	if err != nil {
		return nil, err
	}
	if overridePath == "" {
		return base.Clone(), nil
	}
	override, err := LoadOptional(overridePath)
	if err != nil {
		return nil, err
	}
	return base.Merge(override), nil
}

func (f *File) Clone() *File {
	if f == nil {
		return &File{}
	}
	return f.Merge(nil)
}

func (f *File) Merge(override *File) *File {
	base := &File{}
	if f != nil {
		base.Profile = f.Profile
		base.Profiles = cloneProfiles(f.Profiles)
		base.Plugins = clonePlugins(f.Plugins)
		base.Strictness = f.Strictness
	}
	if override == nil {
		return base
	}

	if override.Profile.Active != "" {
		base.Profile.Active = override.Profile.Active
	}
	if override.Strictness != "" {
		base.Strictness = override.Strictness
	}
	base.Profiles = mergeProfiles(base.Profiles, override.Profiles)
	base.Plugins = mergePlugins(base.Plugins, override.Plugins)
	return base
}

func (f *File) ResolveProfile(explicitFlag string) string {
	if explicitFlag != "" {
		return explicitFlag
	}
	if f == nil {
		return ""
	}
	return f.Profile.Active
}

func (f *File) GetProfile(name string) *Profile {
	if f == nil || name == "" || f.Profiles == nil {
		return nil
	}
	return f.Profiles[name]
}

func (f *File) ValidateProfile(name string) error {
	p := f.GetProfile(name)
	if p == nil {
		return fmt.Errorf("profile %q not found", name)
	}

	known := make(map[string]bool, len(f.Plugins))
	for _, plugin := range f.Plugins {
		known[plugin.ID] = true
	}

	for _, pluginID := range p.Plugins {
		if !known[pluginID] {
			return fmt.Errorf("profile %q references unknown plugin %q", name, pluginID)
		}
	}
	return nil
}

func cloneProfiles(in map[string]*Profile) map[string]*Profile {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]*Profile, len(in))
	for name, profile := range in {
		out[name] = cloneProfile(profile)
	}
	return out
}

func cloneProfile(in *Profile) *Profile {
	if in == nil {
		return &Profile{}
	}
	return &Profile{
		DisplayName: in.DisplayName,
		Description: in.Description,
		Plugins:     cloneStringSlice(in.Plugins),
		Env:         cloneStringMap(in.Env),
	}
}

func mergeProfiles(base, override map[string]*Profile) map[string]*Profile {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	out := cloneProfiles(base)
	if out == nil {
		out = map[string]*Profile{}
	}
	for name, patch := range override {
		existing := out[name]
		out[name] = mergeProfile(existing, patch)
	}
	return out
}

func mergeProfile(base, override *Profile) *Profile {
	out := cloneProfile(base)
	if override == nil {
		return out
	}
	if override.DisplayName != "" {
		out.DisplayName = override.DisplayName
	}
	if override.Description != "" {
		out.Description = override.Description
	}
	if len(override.Plugins) > 0 {
		out.Plugins = cloneStringSlice(override.Plugins)
	}
	out.Env = mergeStringMaps(out.Env, override.Env)
	return out
}

func clonePlugins(in []Plugin) []Plugin {
	if len(in) == 0 {
		return nil
	}
	out := make([]Plugin, len(in))
	for i, plugin := range in {
		out[i] = clonePlugin(plugin)
	}
	return out
}

func clonePlugin(in Plugin) Plugin {
	out := in
	out.Args = cloneStringSlice(in.Args)
	out.Env = cloneStringMap(in.Env)
	out.Commands = append([]protocol.CommandSpec{}, in.Commands...)
	return out
}

func mergePlugins(base, override []Plugin) []Plugin {
	out := clonePlugins(base)
	if len(override) == 0 {
		return out
	}
	indexByID := make(map[string]int, len(out))
	for i, plugin := range out {
		if plugin.ID != "" {
			indexByID[plugin.ID] = i
		}
	}
	for _, patch := range override {
		if i, ok := indexByID[patch.ID]; ok && patch.ID != "" {
			out[i] = mergePlugin(out[i], patch)
			continue
		}
		indexByID[patch.ID] = len(out)
		out = append(out, clonePlugin(patch))
	}
	return out
}

func mergePlugin(base, override Plugin) Plugin {
	out := clonePlugin(base)
	if override.ID != "" {
		out.ID = override.ID
	}
	if override.Path != "" {
		out.Path = override.Path
	}
	if len(override.Args) > 0 {
		out.Args = cloneStringSlice(override.Args)
	}
	if override.Priority != 0 {
		out.Priority = override.Priority
	}
	if override.WorkDir != "" {
		out.WorkDir = override.WorkDir
	}
	if len(override.Commands) > 0 {
		out.Commands = append([]protocol.CommandSpec{}, override.Commands...)
	}
	out.Env = mergeStringMaps(out.Env, override.Env)
	return out
}

func cloneStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mergeStringMaps(base, overlay map[string]string) map[string]string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	out := cloneStringMap(base)
	if out == nil {
		out = map[string]string{}
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}
