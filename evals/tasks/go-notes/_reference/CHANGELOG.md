# Changelog

All notable changes to this project are documented here, following [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added
- `notes stats`: total, oldest/newest creation time and per-tag counts.
- `notes rm <id>...`: remove notes; removing a missing id is reported, not an error.

### Fixed
- `notes ls --tag` now matches tags case-insensitively.

## [0.1.0] - 2026-01-02

### Added
- `notes add` and `notes ls` with `--tag` and `--format human|json`.
