# Tasks

## TODO

- [ ] Phase 1: Config model, local override stacking, and profile resolution
  - [ ] Add `profile`, `profiles`, and optional `.devctl.override.yaml` loading to `pkg/config`
  - [ ] Define explicit `default` profile semantics: only active when selected, not implicit
  - [ ] Add config tests for backward compatibility, merge rules, and profile validation
- [ ] Phase 2: Repository profile filtering
  - [ ] Add profile/override options to `repository.Load`
  - [ ] Filter discovered plugin specs by active profile
  - [ ] Merge profile env into selected plugin specs
  - [ ] Add repository tests for base config, override-defined profiles, and ordering
- [ ] Phase 3: CLI flag and command plumbing
  - [ ] Add shared `--profile` flag to repo commands
  - [ ] Thread profile selection through `RepoContext`, CLI commands, dynamic command loading, and servicecontrol re-planning
  - [ ] Add `devctl profiles list` and `devctl profiles active`
- [ ] Phase 4: State file profile recording
  - [ ] Add `profile` field to state
  - [ ] Record active profile on `up`
  - [ ] Show profile in `status`/`down` where appropriate
- [ ] Phase 5: Integration tests and smoke validation
  - [ ] Add multi-profile test fixtures
  - [ ] Test no-profile backward compatibility with top-level plugins
  - [ ] Test explicit `default` profile behavior
  - [ ] Test `.devctl.override.yaml` profile additions and adjustments
- [ ] Phase 6: Documentation finalization
  - [ ] Update DCTL-PROFILES design, diary, and changelog with implementation results
  - [ ] Run `docmgr doctor`
  - [ ] Upload final version to reMarkable if requested

## DONE

- [x] Created DCTL-PROFILES investigation/design ticket
- [x] Uploaded initial design doc to reMarkable
- [x] Updated design to include `.devctl.override.yaml` local stacking
- [x] Clarified explicit `default` profile semantics and no-profile backward compatibility
- [x] Uploaded v2 design doc to reMarkable
