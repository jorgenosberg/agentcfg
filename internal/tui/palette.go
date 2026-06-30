package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgenosberg/agentcfg/internal/backup"
	"github.com/jorgenosberg/agentcfg/internal/claudecfg"
	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/lock"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
)

func (m model) buildItemActions() []paletteAction {
	switch m.mode {
	case viewAgentcfg:
		return m.buildSourceItemActions()
	case viewAgentFolders:
		return m.buildAgentItemActions()
	case viewProjects:
		return m.buildProjectItemActions()
	case viewPlugins:
		return m.buildPluginItemActions()
	}
	return nil
}

func (m model) buildSourceItemActions() []paletteAction {
	grouped := m.groupedItems()
	if m.cursor >= len(grouped) {
		return nil
	}
	g := grouped[m.cursor]
	cfg := m.cfg
	cfgPath := m.cfgPath

	var hasAbsent, hasLinkedCopied, hasDrifted, hasUnmanaged bool
	for _, e := range g.Entries {
		switch e.Status {
		case sync.StatusAbsent:
			hasAbsent = true
		case sync.StatusLinked, sync.StatusCopied:
			hasLinkedCopied = true
		case sync.StatusDrifted:
			hasDrifted = true
			hasLinkedCopied = true
		case sync.StatusUnmanaged:
			hasUnmanaged = true
		}
	}

	var actions []paletteAction

	if hasAbsent || hasDrifted {
		entries := g.Entries
		item := g.Item
		actions = append(actions, paletteAction{
			label: "Install to all targets",
			fn: func() (overlayModel, tea.Cmd) {
				return nil, func() tea.Msg {
					var ok, fail int
					for _, e := range entries {
						if _, err := sync.Install(e.Target, e.Target.ResolveStrategy(cfg.DefaultStrategy), e.Item); err != nil {
							fail++
						} else {
							ok++
						}
					}
					if fail > 0 {
						return cfgReloadMsg{status: fmt.Sprintf("installed %d, %d errors", ok, fail)}
					}
					return cfgReloadMsg{status: fmt.Sprintf("installed %s (%d targets)", item.Name, ok)}
				}
			},
		})
	}

	// Toggle — always offered
	{
		item := g.Item
		targets := cfg.Targets
		if m.sourceTarget != "" {
			for _, t := range cfg.Targets {
				if t.Name == m.sourceTarget {
					targets = []config.Target{t}
					break
				}
			}
		}
		allDisabled := true
		for _, t := range targets {
			if !t.IsDisabled(item) {
				allDisabled = false
				break
			}
		}
		disable := !allDisabled
		label := "Disable item"
		if allDisabled {
			label = "Enable item"
		}
		tgts := targets
		actions = append(actions, paletteAction{
			label: label,
			fn: func() (overlayModel, tea.Cmd) {
				return nil, func() tea.Msg {
					for _, t := range tgts {
						if err := sync.Toggle(cfgPath, t.Name, item, disable); err != nil {
							return cfgReloadMsg{err: err}
						}
					}
					return cfgReloadMsg{status: fmt.Sprintf("toggled %s", item.Name)}
				}
			},
		})
	}

	if hasUnmanaged {
		entries := g.Entries
		item := g.Item
		actions = append(actions, paletteAction{
			label: "Adopt unmanaged file",
			fn: func() (overlayModel, tea.Cmd) {
				return nil, func() tea.Msg {
					var ok, fail int
					for _, e := range entries {
						if e.Status != sync.StatusUnmanaged {
							continue
						}
						if _, err := sync.Adopt(e.Target, e.Target.ResolveStrategy(cfg.DefaultStrategy), e.Item); err != nil {
							fail++
						} else {
							ok++
						}
					}
					if fail > 0 {
						return cfgReloadMsg{status: fmt.Sprintf("adopted %d, %d errors", ok, fail)}
					}
					return cfgReloadMsg{status: fmt.Sprintf("adopted %s (%d targets)", item.Name, ok)}
				}
			},
		})
	}

	if hasLinkedCopied {
		entries := g.Entries
		item := g.Item
		actions = append(actions, paletteAction{
			label: "Uninstall from all targets",
			fn: func() (overlayModel, tea.Cmd) {
				return newConfirmOverlay(
					fmt.Sprintf("Uninstall %q?", item.Name),
					"Removes installed files from all targets.",
					func() error {
						var lastErr error
						for _, e := range entries {
							if err := sync.Uninstall(e.Target, e.Target.ResolveStrategy(cfg.DefaultStrategy), e.Item); err != nil {
								lastErr = err
							}
						}
						return lastErr
					},
				), nil
			},
		})

		targets := cfg.Targets
		if m.sourceTarget != "" {
			for _, t := range cfg.Targets {
				if t.Name == m.sourceTarget {
					targets = []config.Target{t}
					break
				}
			}
		}
		tgts := targets
		targetCount := len(tgts)
		detail := fmt.Sprintf("Place a real copy in %d target dir(s) and stop managing. File stays in source.", targetCount)
		if targetCount == 1 {
			detail = fmt.Sprintf("Place a real copy in %s and stop managing. File stays in source.", tgts[0].Path)
		}
		actions = append(actions, paletteAction{
			label: "Unmanage",
			fn: func() (overlayModel, tea.Cmd) {
				return newConfirmOverlay(
					fmt.Sprintf("Unmanage %q?", item.Name),
					detail,
					func() error {
						for _, t := range tgts {
							if err := sync.Unmanage(t, t.ResolveStrategy(cfg.DefaultStrategy), item); err != nil {
								return err
							}
							if err := sync.Toggle(cfgPath, t.Name, item, true); err != nil {
								return err
							}
						}
						return nil
					},
				), nil
			},
		})
	}

	// Editor and versioning — always shown for source items
	{
		item := g.Item
		actions = append(actions, paletteAction{
			label: "Open in editor",
			fn: func() (overlayModel, tea.Cmd) {
				editor := os.Getenv("VISUAL")
				if editor == "" {
					editor = os.Getenv("EDITOR")
				}
				if editor == "" {
					editor = "vi"
				}
				c := exec.Command(editor, item.Path)
				return nil, tea.ExecProcess(c, func(err error) tea.Msg {
					if err != nil {
						return cfgReloadMsg{err: err}
					}
					return cfgReloadMsg{status: "editor closed"}
				})
			},
		})

		actions = append(actions, paletteAction{
			label: "Save as version",
			fn: func() (overlayModel, tea.Cmd) {
				o, cmd := newSaveVersionOverlay(item)
				return o, cmd
			},
		})

		versions, _ := source.ListVersions(item.Path)
		if len(versions) > 0 {
			versionActions := make([]paletteAction, len(versions))
			for i, v := range versions {
				versionActions[i] = paletteAction{
					label: v,
					fn: func() (overlayModel, tea.Cmd) {
						return nil, func() tea.Msg {
							if err := source.SwitchVersion(item.Path, v); err != nil {
								return cfgReloadMsg{err: err}
							}
							return cfgReloadMsg{status: fmt.Sprintf("switched to version %q", v)}
						}
					},
				}
			}
			actions = append(actions, paletteAction{
				label: fmt.Sprintf("Switch version (%d saved)", len(versions)),
				fn: func() (overlayModel, tea.Cmd) {
					return newPaletteOverlay("Switch version", versionActions), nil
				},
			})

			deleteActions := make([]paletteAction, len(versions))
			for i, v := range versions {
				deleteActions[i] = paletteAction{
					label: v,
					fn: func() (overlayModel, tea.Cmd) {
						return newConfirmOverlay(
							fmt.Sprintf("Delete version %q?", v),
							"Removes the saved version. The active item is unchanged.",
							func() error { return source.DeleteVersion(item.Path, v) },
						), nil
					},
				}
			}
			actions = append(actions, paletteAction{
				label: "Delete version",
				fn: func() (overlayModel, tea.Cmd) {
					return newPaletteOverlay("Delete version", deleteActions), nil
				},
			})
		}
	}

	return actions
}

func (m model) buildAgentItemActions() []paletteAction {
	entries := m.filteredTargetEntries()
	if m.cursor >= len(entries) {
		return nil
	}
	e := entries[m.cursor]
	dest := e.Dest

	if e.Status == sync.StatusUnmanaged {
		return []paletteAction{{
			label: "Delete (not managed by agentcfg)",
			fn: func() (overlayModel, tea.Cmd) {
				return newConfirmOverlay(
					fmt.Sprintf("Delete %q from %s?", e.Item.Name, e.Target.Name),
					fmt.Sprintf("Not managed by agentcfg. Permanently deletes:\n%s", dest),
					func() error { return os.RemoveAll(dest) },
				), nil
			},
		}}
	}

	return []paletteAction{{
		label: fmt.Sprintf("Remove from %s", e.Target.Name),
		fn: func() (overlayModel, tea.Cmd) {
			return newConfirmOverlay(
				fmt.Sprintf("Remove %q from %s?", e.Item.Name, e.Target.Name),
				fmt.Sprintf("Removes installed file:\n%s", dest),
				func() error { return os.RemoveAll(dest) },
			), nil
		},
	}}
}

func (m model) buildProjectItemActions() []paletteAction {
	if m.cursor >= len(m.projectItems) {
		return nil
	}
	projName := m.projectItems[m.cursor].Project
	cfgPath := m.cfgPath
	cfg := m.cfg

	return []paletteAction{{
		label: fmt.Sprintf("Remove project %q from config", projName),
		fn: func() (overlayModel, tea.Cmd) {
			return newConfirmOverlay(
				fmt.Sprintf("Remove project %q?", projName),
				"Removes from config only. No files are deleted.",
				func() error {
					out := make([]config.Project, 0, len(cfg.Projects))
					for _, p := range cfg.Projects {
						if p.Name != projName {
							out = append(out, p)
						}
					}
					cfg.Projects = out
					return config.Save(cfgPath, cfg)
				},
			), nil
		},
	}}
}

func (m model) buildPluginItemActions() []paletteAction {
	if m.pluginReg == nil || m.cursor >= len(m.pluginReg.Plugins) {
		return nil
	}
	p := m.pluginReg.Plugins[m.cursor]
	ff := m.forkFile
	forksRoot := m.forksRoot
	forksPath := m.forksPath
	settingsPath := m.settingsPath
	knownMPPath := m.knownMPPath
	installedPluginsPath := m.installedPluginsPath

	var actions []paletteAction

	if !ff.PluginForked(p.FullName) && p.Installed {
		plugin := p
		actions = append(actions, paletteAction{
			label: "Fork plugin (own a full copy)",
			fn: func() (overlayModel, tea.Cmd) {
				return newForkOverlay(plugin, forksRoot, forksPath, settingsPath, knownMPPath, installedPluginsPath), nil
			},
		})
	}

	if p.Enabled {
		plugin := p
		actions = append(actions, paletteAction{
			label: "Disable plugin",
			fn: func() (overlayModel, tea.Cmd) {
				return newConfirmOverlay(
					fmt.Sprintf("Disable %q?", plugin.FullName),
					"Prevents Claude Code from loading this plugin.",
					func() error {
						return claudecfg.SetPluginEnabled(settingsPath, plugin.FullName, false)
					},
				), nil
			},
		})
	} else {
		plugin := p
		actions = append(actions, paletteAction{
			label: "Enable plugin",
			fn: func() (overlayModel, tea.Cmd) {
				return newConfirmOverlay(
					fmt.Sprintf("Enable %q?", plugin.FullName),
					"Allows Claude Code to load this plugin.",
					func() error {
						return claudecfg.SetPluginEnabled(settingsPath, plugin.FullName, true)
					},
				), nil
			},
		})
	}

	return actions
}

func (m model) buildGlobalActions() []paletteAction {
	cfg := m.cfg
	cfgPath := m.cfgPath
	items := m.items

	var actions []paletteAction

	// Sync all
	actions = append(actions, paletteAction{
		label: "Sync all",
		fn: func() (overlayModel, tea.Cmd) {
			return nil, func() tea.Msg {
				lockPath, err := lock.DefaultPath()
				if err != nil {
					return cfgReloadMsg{err: err}
				}
				lck, err := lock.Load(lockPath)
				if err != nil {
					return cfgReloadMsg{err: err}
				}
				results := sync.Sync(cfg, items, lck, false, false)
				var installed, updated int
				for _, r := range results {
					if r.Err == nil {
						if r.OldStatus == sync.StatusAbsent {
							installed++
						} else {
							updated++
						}
					}
				}
				if len(results) > 0 {
					_ = lock.Save(lockPath, lck)
				}
				status := "everything up to date"
				if len(results) > 0 {
					status = fmt.Sprintf("sync: %d installed, %d updated", installed, updated)
				}
				return cfgReloadMsg{status: status}
			}
		},
	})

	// Rescan
	actions = append(actions, paletteAction{
		label: "Rescan",
		fn: func() (overlayModel, tea.Cmd) {
			return nil, func() tea.Msg { return cfgReloadMsg{status: "rescanned"} }
		},
	})

	actions = append(actions, paletteAction{
		label: "Create backup",
		fn: func() (overlayModel, tea.Cmd) {
			return nil, func() tea.Msg {
				root, err := backup.DefaultRoot()
				if err != nil {
					return cfgReloadMsg{err: err}
				}
				dir, err := backup.Create(cfg, root)
				if err != nil {
					return cfgReloadMsg{err: err}
				}
				_ = backup.Prune(root, 5)
				return cfgReloadMsg{status: fmt.Sprintf("backup: %s", filepath.Base(dir))}
			}
		},
	})

	// Restore from latest backup
	actions = append(actions, paletteAction{
		label: "Restore from latest backup",
		fn: func() (overlayModel, tea.Cmd) {
			return newConfirmOverlay(
				"Restore from latest backup?",
				"Overwrites current target directories with the most recent snapshot.",
				func() error {
					root, err := backup.DefaultRoot()
					if err != nil {
						return err
					}
					snaps, err := backup.List(root)
					if err != nil {
						return err
					}
					if len(snaps) == 0 {
						return fmt.Errorf("no backups found")
					}
					entries, err := os.ReadDir(root)
					if err != nil {
						return err
					}
					ts := snaps[0].Timestamp.UTC().Format("20060102-150405")
					var snapshotDir string
					for _, e := range entries {
						if e.Name() == ts {
							snapshotDir = filepath.Join(root, e.Name())
							break
						}
					}
					if snapshotDir == "" {
						return fmt.Errorf("snapshot directory not found")
					}
					return backup.Restore(snapshotDir, cfg)
				},
			), nil
		},
	})

	actions = append(actions, paletteAction{
		label: "Add target",
		fn: func() (overlayModel, tea.Cmd) {
			o, cmd := newAddTargetOverlay(cfgPath, cfg)
			return o, cmd
		},
	})

	// Discover agents
	actions = append(actions, paletteAction{
		label: "Discover agents",
		fn: func() (overlayModel, tea.Cmd) {
			return newDiscoverOverlay(cfgPath, cfg), nil
		},
	})

	// Remove target — only shown if a target filter is currently active
	targetName := m.sourceTarget
	if targetName == "" {
		targetName = m.targetFilter
	}
	if targetName != "" {
		t := targetName
		actions = append(actions, paletteAction{
			label: fmt.Sprintf("Remove target %q", t),
			fn: func() (overlayModel, tea.Cmd) {
				return newConfirmOverlay(
					fmt.Sprintf("Remove target %q?", t),
					"Removes from config only. Installed items are not uninstalled.",
					func() error {
						out := make([]config.Target, 0, len(cfg.Targets))
						for _, tgt := range cfg.Targets {
							if tgt.Name != t {
								out = append(out, tgt)
							}
						}
						cfg.Targets = out
						return config.Save(cfgPath, cfg)
					},
				), nil
			},
		})
	}

	actions = append(actions, paletteAction{
		label: "Add project",
		fn: func() (overlayModel, tea.Cmd) {
			o, cmd := newAddProjectOverlay(cfgPath, cfg)
			return o, cmd
		},
	})

	// Remove project — only shown when cursor is on a project item
	if m.mode == viewProjects && m.cursor < len(m.projectItems) {
		projName := m.projectItems[m.cursor].Project
		p := projName
		actions = append(actions, paletteAction{
			label: fmt.Sprintf("Remove project %q", p),
			fn: func() (overlayModel, tea.Cmd) {
				return newConfirmOverlay(
					fmt.Sprintf("Remove project %q?", p),
					"Removes from config only. No files are deleted.",
					func() error {
						out := make([]config.Project, 0, len(cfg.Projects))
						for _, proj := range cfg.Projects {
							if proj.Name != p {
								out = append(out, proj)
							}
						}
						cfg.Projects = out
						return config.Save(cfgPath, cfg)
					},
				), nil
			},
		})
	}

	// Init wizard
	actions = append(actions, paletteAction{
		label: "Init wizard",
		fn: func() (overlayModel, tea.Cmd) {
			o, cmd := newInitWizardOverlay(cfgPath)
			return o, cmd
		},
	})

	return actions
}
