// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

// network_facade.go previously held thin method wrappers that delegated
// to package-level free functions. The real method bodies now live in
// net.go (and sibling files) operating on Network fields directly; the
// package-level free functions delegate to defaultNet.
