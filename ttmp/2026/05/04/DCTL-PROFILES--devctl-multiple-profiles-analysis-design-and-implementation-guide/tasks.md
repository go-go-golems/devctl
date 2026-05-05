# Tasks

## TODO

- [x] Phase 1: Config model, local override stacking, and profile resolution
  - [x] Add `profile`, `profiles`, and optional `.devctl.override.yaml` loading to `pkg/config`
  - [x] Define explicit `default` profile semantics: only active when selected, not implicit
  - [x] Add config tests for backward compatibility, merge rules, and profile validation
- [x] Phase 2: Repository profile filtering
  - [x] Add profile/override options to `repository.Load`
  - [x] Filter discovered plugin specs by active profile
  - [x] Merge profile env into selected plugin specs
  - [x] Add repository tests for base config, override-defined profiles, and ordering
- [x] Phase 3: CLI flag and command plumbing
  - [x] Add shared `--profile` flag to repo commands
  - [x] Thread profile selection through `RepoContext`, CLI commands, dynamic command loading, and servicecontrol re-planning
  - [x] Add `devctl profiles list` and `devctl profiles active`
- [x] Phase 4: State file profile recording
  - [x] Add `profile` field to state
  - [x] Record active profile on `up`
  - [x] Show profile in `status`/`down` where appropriate
- [x] Phase 5: Integration tests and smoke validation
  - [x] Add multi-profile test fixtures
  - [x] Test no-profile backward compatibility with top-level plugins
  - [x] Test explicit `default` profile behavior
  - [x] Test `.devctl.override.yaml` profile additions and adjustments
- [x] Phase 6: Documentation finalization
  - [x] Update DCTL-PROFILES design, diary, and changelog with implementation results
  - [x] Run `docmgr doctor`
  - [ ] Upload final version to reMarkable if requested

## DONE

- [x] Created DCTL-PROFILES investigation/design ticket
- [x] Uploaded initial design doc to reMarkable
- [x] Updated design to include `.devctl.override.yaml` local stacking
- [x] Clarified explicit `default` profile semantics and no-profile backward compatibility
- [x] Uploaded v2 design doc to reMarkable
